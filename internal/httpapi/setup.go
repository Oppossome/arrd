package httpapi

import (
	"context"

	"generic-arr-project/internal/httpapi/apigen"
	"generic-arr-project/internal/library"
)

func (h *handlers) SetupStatus(context.Context, apigen.SetupStatusRequestObject) (apigen.SetupStatusResponseObject, error) {
	return apigen.SetupStatus200JSONResponse(toAPISetupStatus(h.svc.SetupStatus())), nil
}

func (h *handlers) CompleteSetup(ctx context.Context, req apigen.CompleteSetupRequestObject) (apigen.CompleteSetupResponseObject, error) {
	out, err := h.svc.CompleteSetup(ctx, fromAPISetupInput(*req.Body))
	if err != nil {
		return nil, err
	}
	// The wizard ends signed in.
	cookie := sessionCookieFor(httpRequest(ctx), out.SessionToken).String()
	return apigen.CompleteSetup201JSONResponse{
		Body:    toAPISetupResult(out),
		Headers: apigen.CompleteSetup201ResponseHeaders{SetCookie: &cookie},
	}, nil
}

func (h *handlers) ValidatePaths(_ context.Context, req apigen.ValidatePathsRequestObject) (apigen.ValidatePathsResponseObject, error) {
	return apigen.ValidatePaths200JSONResponse(toAPIPathsCheck(library.ValidatePaths(fromAPIPathsInput(*req.Body)))), nil
}

func (h *handlers) ValidateVPN(ctx context.Context, req apigen.ValidateVPNRequestObject) (apigen.ValidateVPNResponseObject, error) {
	in := fromAPIVPNInput(*req.Body)
	out, err := h.svc.TestVPN(ctx, &in)
	if err != nil {
		return nil, err
	}
	return apigen.ValidateVPN200JSONResponse(toAPIVPNTest(out)), nil
}

func (h *handlers) ValidateTMDB(ctx context.Context, req apigen.ValidateTMDBRequestObject) (apigen.ValidateTMDBResponseObject, error) {
	if err := h.svc.ValidateTMDBKey(ctx, req.Body.TMDBAPIKey); err != nil {
		return nil, err
	}
	return apigen.ValidateTMDB200JSONResponse{Valid: true}, nil
}

func toAPISetupStatus(s library.SetupStatus) apigen.SetupStatus {
	return apigen.SetupStatus{
		SetupComplete: s.SetupComplete,
		Suggested: apigen.SuggestedPaths{
			IncompleteDir: s.Suggested.IncompleteDir,
			CompleteDir:   s.Suggested.CompleteDir,
			MovieRoot:     s.Suggested.MovieRoot,
			TVRoot:        s.Suggested.TVRoot,
		},
	}
}

func fromAPISetupInput(in apigen.SetupInput) library.SetupInput {
	out := library.SetupInput{
		AdminPassword: in.AdminPassword,
		TMDBAPIKey:    in.TMDBAPIKey,
		Downloads:     fromAPIDownloadSettingsInput(in.Downloads),
		MovieRoot:     deref(in.MovieRoot),
		TVRoot:        deref(in.TVRoot),
		Indexers:      mapSlice(derefSlice(in.Indexers), fromAPIIndexerInput),
		CreateMissing: deref(in.CreateMissing),
	}
	if in.VPN != nil {
		v := fromAPIVPNInput(*in.VPN)
		out.VPN = &v
	}
	return out
}

func toAPISetupResult(r library.SetupResult) apigen.SetupResult {
	return apigen.SetupResult{
		APIKey:       r.APIKey,
		Warnings:     optSlice(r.Warnings, identity[string]),
		RuntimeError: optional(r.RuntimeError),
	}
}

func fromAPIPathsInput(in apigen.PathsInput) library.PathsInput {
	return library.PathsInput{
		Downloads:     fromAPIDownloadSettingsInput(in.Downloads),
		MovieRoot:     deref(in.MovieRoot),
		TVRoot:        deref(in.TVRoot),
		CreateMissing: deref(in.CreateMissing),
	}
}

func toAPIPathsCheck(c library.PathsCheck) apigen.PathsCheck {
	errs := c.Errors
	if errs == nil {
		errs = map[string]string{}
	}
	return apigen.PathsCheck{Valid: c.Valid, Errors: errs, Warnings: nonNil(c.Warnings)}
}
