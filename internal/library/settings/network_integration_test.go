package settings_test

import (
	"context"
	"errors"
	"testing"

	"generic-arr-project/internal/library/libtest"
	"generic-arr-project/internal/library/settings"
)

// fakeApplier records ApplyLAN calls and fails while err is set.
type fakeApplier struct {
	lan   bool
	calls []bool
	err   error
}

func (f *fakeApplier) ApplyLAN(enabled bool) error {
	f.calls = append(f.calls, enabled)
	if f.err != nil {
		return f.err
	}
	f.lan = enabled
	return nil
}

func (f *fakeApplier) Hostname() string { return "arrd.local" }

func (f *fakeApplier) LANURL() string {
	if !f.lan {
		return ""
	}
	return "http://arrd.local:8484"
}

func TestNetworkSettings(t *testing.T) {
	ctx := context.Background()
	base := t.TempDir()
	env := libtest.NewEnv(t, base, "", libtest.BasicSetup(base), nil)

	if _, err := env.Svc.Settings.NetworkSettings(ctx); err == nil {
		t.Error("NetworkSettings without an applier succeeded")
	}
	if _, err := env.Svc.Settings.UpdateNetworkSettings(ctx, settings.NetworkSettingsInput{ExposeLAN: true}); err == nil {
		t.Error("UpdateNetworkSettings without an applier succeeded")
	}

	applier := &fakeApplier{}
	env.Svc.Settings.SetNetworkApplier(applier)
	got, err := env.Svc.Settings.NetworkSettings(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if want := (settings.NetworkSettings{Hostname: "arrd.local"}); got != want {
		t.Errorf("initial = %+v, want %+v", got, want)
	}

	got, err = env.Svc.Settings.UpdateNetworkSettings(ctx, settings.NetworkSettingsInput{ExposeLAN: true})
	if err != nil {
		t.Fatal(err)
	}
	if want := (settings.NetworkSettings{ExposeLAN: true, Hostname: "arrd.local", LANURL: "http://arrd.local:8484"}); got != want {
		t.Errorf("enabled = %+v, want %+v", got, want)
	}
	if app, _ := env.Store.GetAppSettings(ctx); !app.ExposeLAN {
		t.Error("enabled setting not saved")
	}
}

func TestNetworkSettingsApplyFailureSavesNothing(t *testing.T) {
	ctx := context.Background()
	base := t.TempDir()
	env := libtest.NewEnv(t, base, "", libtest.BasicSetup(base), nil)
	applier := &fakeApplier{err: errors.New("address already in use")}
	env.Svc.Settings.SetNetworkApplier(applier)

	if _, err := env.Svc.Settings.UpdateNetworkSettings(ctx, settings.NetworkSettingsInput{ExposeLAN: true}); err == nil {
		t.Fatal("update succeeded although apply failed")
	}
	if app, _ := env.Store.GetAppSettings(ctx); app.ExposeLAN {
		t.Error("setting saved although apply failed")
	}
}
