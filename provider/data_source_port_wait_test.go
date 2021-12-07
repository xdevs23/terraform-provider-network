package provider

import (
	"net/http"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestTryConnect(t *testing.T) {
	diags := diag.Diagnostics{}
	d := dataSourcePortWait{}
	d.tryConnect(
		&dataSourcePortWaitType{
			Host:           types.String{Value: "nonexistent.example.tld"},
			Port:           types.Int64{Value: 80},
			TimeoutSec:     types.Int64{Value: 1},
			ErrorOnTimeout: types.Bool{Value: true},
		},
		&diags,
	)
	if !diags.HasError() {
		t.Fatal("expected error")
	}

	diags = diag.Diagnostics{}

	state := dataSourcePortWaitType{
		Host:           types.String{Value: "nonexistent.example.tld"},
		Port:           types.Int64{Value: 22},
		TimeoutSec:     types.Int64{Value: 1},
		ErrorOnTimeout: types.Bool{Value: false},
		CooldownMs:     types.Int64{Value: 10},
	}
	d.tryConnect(&state, &diags)
	if diags.HasError() {
		t.Fatal("did not expect error")
	}
	if state.Available.Value {
		t.Fatal("expected Available to be false")
	}

	diags = diag.Diagnostics{}

	d.tryConnect(
		&dataSourcePortWaitType{
			Host:       types.String{Value: "1.1.1.1"},
			Port:       types.Int64{Value: 443},
			TimeoutSec: types.Int64{Value: 5},
		},
		&diags,
	)
	if diags.HasError() {
		t.Log(diags)
		t.Fatal("expected connection to succeed")
	}

	diags = diag.Diagnostics{}

	srv := http.Server{}
	srv.Addr = ":4819"
	srvErr := make(chan error)
	go func() {
		err := srv.ListenAndServe()
		if err != nil && err != http.ErrServerClosed {
			srvErr <- err
		} else {
			srvErr <- nil
		}
		close(srvErr)
	}()

	state = dataSourcePortWaitType{
		Host:       types.String{Value: "127.0.0.1"},
		Port:       types.Int64{Value: 4819},
		TimeoutSec: types.Int64{Value: 1},
	}
	d.tryConnect(
		&state,
		&diags,
	)
	srv.Close()
	if <-srvErr != nil {
		t.Error(srvErr)
	}
	if diags.HasError() {
		t.Log(diags)
		t.Fatal("expected connection to succeed")
	}
	if !state.Available.Value {
		t.Fatal("expected service to be available but it isn't")
	}
}
