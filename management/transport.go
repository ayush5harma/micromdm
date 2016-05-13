package management

import (
	"encoding/json"
	"io"
	"net/http"

	kitlog "github.com/go-kit/kit/log"
	kithttp "github.com/go-kit/kit/transport/http"
	"github.com/gorilla/mux"
	"github.com/micromdm/micromdm/profile"
	"golang.org/x/net/context"
)

// ServiceHandler returns an HTTP Handler for the management service
func ServiceHandler(ctx context.Context, svc Service, logger kitlog.Logger) http.Handler {
	opts := []kithttp.ServerOption{
		kithttp.ServerErrorLogger(logger),
		kithttp.ServerErrorEncoder(encodeError),
	}

	fetchDEPHandler := kithttp.NewServer(
		ctx,
		makeFetchDevicesEndpoint(svc),
		decodeFetchDEPDevicesRequest,
		encodeResponse,
		opts...,
	)

	addProfileHandler := kithttp.NewServer(
		ctx,
		makeAddProfileEndpoint(svc),
		decodeAddProfileRequest,
		encodeResponse,
		opts...,
	)
	listProfilesHandler := kithttp.NewServer(
		ctx,
		makeListProfilesEndpoint(svc),
		decodeListProfilesRequest,
		encodeResponse,
		opts...,
	)

	r := mux.NewRouter()

	r.Handle("/management/v1/devices/fetch", fetchDEPHandler).Methods("POST")
	r.Handle("/management/v1/profiles", addProfileHandler).Methods("POST")
	r.Handle("/management/v1/profiles", listProfilesHandler).Methods("GET")

	return r
}

func decodeFetchDEPDevicesRequest(_ context.Context, r *http.Request) (interface{}, error) {
	return fetchDEPDevicesRequest{}, nil
}

func decodeAddProfileRequest(_ context.Context, r *http.Request) (interface{}, error) {
	var request addProfileRequest
	err := json.NewDecoder(r.Body).Decode(&request)
	if err == io.EOF {
		return nil, errEmptyRequest
	}
	if request.PayloadIdentifier == "" {
		return nil, errEmptyRequest
	}
	return request, err
}

func decodeListProfilesRequest(_ context.Context, r *http.Request) (interface{}, error) {
	return listProfilesRequest{}, nil
}

func encodeResponse(ctx context.Context, w http.ResponseWriter, response interface{}) error {
	if e, ok := response.(errorer); ok && e.error() != nil {
		encodeError(ctx, e.error(), w)
		return nil
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	// for success responses
	if e, ok := response.(statuser); ok {
		w.WriteHeader(e.status())
	}

	// check if this is a collection
	if e, ok := response.(listEncoder); ok {
		return e.encodeList(w)

	}
	return json.NewEncoder(w).Encode(response)
}

type errorer interface {
	error() error
}

type statuser interface {
	status() int
}

type listEncoder interface {
	encodeList(w http.ResponseWriter) error
}

// encode errors from business-logic
func encodeError(_ context.Context, err error, w http.ResponseWriter) {
	// unwrap if the error is wrapped by kit http in it's own error type
	if httperr, ok := err.(kithttp.Error); ok {
		err = httperr.Err
	}
	switch err {
	case errEmptyRequest:
		w.WriteHeader(http.StatusBadRequest)
	case profile.ErrExists:
		w.WriteHeader(http.StatusConflict)
	default:
		w.WriteHeader(http.StatusInternalServerError)
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"error": err.Error(),
	})
}