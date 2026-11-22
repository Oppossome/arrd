package httpapi

import (
	"context"

	"generic-arr-project/internal/httpapi/apigen"
	"generic-arr-project/internal/library"
	"generic-arr-project/internal/vpn/provider"
)

func (h *handlers) GetVPNSettings(ctx context.Context, _ apigen.GetVPNSettingsRequestObject) (apigen.GetVPNSettingsResponseObject, error) {
	out, err := h.svc.VPNSettings(ctx)
	if err != nil {
		return nil, err
	}
	return apigen.GetVPNSettings200JSONResponse(toAPIVPNSettings(out)), nil
}

func (h *handlers) PutVPNSettings(ctx context.Context, req apigen.PutVPNSettingsRequestObject) (apigen.PutVPNSettingsResponseObject, error) {
	out, err := h.svc.SetVPN(ctx, fromAPIVPNInput(*req.Body))
	if err != nil {
		return nil, err
	}
	return apigen.PutVPNSettings200JSONResponse(toAPIVPNSettings(out)), nil
}

func (h *handlers) DeleteVPNSettings(ctx context.Context, _ apigen.DeleteVPNSettingsRequestObject) (apigen.DeleteVPNSettingsResponseObject, error) {
	if err := h.svc.RemoveVPN(ctx); err != nil {
		return nil, err
	}
	return apigen.DeleteVPNSettings204Response{}, nil
}

// TestVPN tests the posted provider settings, or the running tunnel for an
// empty body.
func (h *handlers) TestVPN(ctx context.Context, req apigen.TestVPNRequestObject) (apigen.TestVPNResponseObject, error) {
	var in *library.VPNInput
	if req.Body != nil {
		v := fromAPIVPNInput(*req.Body)
		in = &v
	}
	out, err := h.svc.TestVPN(ctx, in)
	if err != nil {
		return nil, err
	}
	return apigen.TestVPN200JSONResponse(toAPIVPNTest(out)), nil
}

// VPNProviders lists VPN providers and the settings each needs; used by
// settings and the setup wizard.
func (h *handlers) VPNProviders(context.Context, apigen.VPNProvidersRequestObject) (apigen.VPNProvidersResponseObject, error) {
	return apigen.VPNProviders200JSONResponse(mapSlice(h.svc.VPNProviders(), toAPIVPNProvider)), nil
}

// VPNOptions lists the choices for a provider's select field.
func (h *handlers) VPNOptions(ctx context.Context, req apigen.VPNOptionsRequestObject) (apigen.VPNOptionsResponseObject, error) {
	out, err := h.svc.VPNOptions(ctx, req.ID, req.Field)
	if err != nil {
		return nil, err
	}
	return apigen.VPNOptions200JSONResponse(mapSlice(out, toAPISettingOption)), nil
}

func toAPIVPNProvider(d provider.Definition) apigen.VPNProvider {
	return apigen.VPNProvider{
		ID:          d.ID,
		Name:        d.Name,
		Description: optional(d.Description),
		Fields:      mapSlice(d.Fields, toAPISettingField),
		Renews:      d.Renews,
	}
}

func fromAPIVPNInput(in apigen.VPNInput) library.VPNInput {
	return library.VPNInput{Provider: in.Provider, Values: provider.Values(in.Values)}
}

func toAPIVPNSettings(s library.VPNSettings) apigen.VPNSettings {
	out := apigen.VPNSettings{
		Configured:    s.Configured,
		Provider:      optional(s.Provider),
		SecretsSet:    optSlice(s.SecretsSet, identity[string]),
		TunnelUp:      s.TunnelUp,
		LastHandshake: s.LastHandshake,
		TunnelError:   optional(s.TunnelError),
		Server:        optional(s.Server),
		ForwardedPort: optional(s.ForwardedPort),
		Addresses:     optSlice(s.Addresses, identity[string]),
		Endpoints:     optSlice(s.Endpoints, identity[string]),
		DNS:           optSlice(s.DNS, identity[string]),
		IgnoredKeys:   optSlice(s.IgnoredKeys, identity[string]),
	}
	if len(s.Values) > 0 {
		v := apigen.SettingValues(s.Values)
		out.Values = &v
	}
	return out
}

func toAPIVPNTest(t library.VPNTest) apigen.VPNTest {
	return apigen.VPNTest{
		PublicIP:      t.PublicIP,
		IgnoredKeys:   optSlice(t.IgnoredKeys, identity[string]),
		DNSConfigured: t.HasDNS,
	}
}
