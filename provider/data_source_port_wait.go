package provider

import (
	"context"
	"fmt"
	"net"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type dataSourcePortWaitType struct {
	ID             types.String `tfsdk:"id"`
	Available      types.Bool   `tfsdk:"available"`
	TimeoutSec     types.Int64  `tfsdk:"timeout_sec"`
	ErrorOnTimeout types.Bool   `tfsdk:"error_on_timeout"`
	Host           types.String `tfsdk:"host"`
	Port           types.Int64  `tfsdk:"port"`
	CooldownMs     types.Int64  `tfsdk:"cooldown_ms"`
}

const defaultCooldownMs int64 = 500

func (d dataSourcePortWaitType) GetSchema(_ context.Context) (tfsdk.Schema, diag.Diagnostics) {
	return tfsdk.Schema{
		Attributes: map[string]tfsdk.Attribute{
			"id": {
				Type:     types.StringType,
				Computed: true,
			},
			"available": {
				Description: "Whether the port is available after timeout",
				Type:        types.BoolType,
				Computed:    true,
			},
			"timeout_sec": {
				Description: "How many seconds to wait before timing out. 0 means infinite (default).",
				Type:        types.Int64Type,
				Optional:    true,
			},
			"error_on_timeout": {
				Description: "Treats a timeout as error causing the plan/apply operation to fail",
				Type:        types.BoolType,
				Optional:    true,
			},
			"host": {
				Description: "Hostname, domain name, IP address",
				Type:        types.StringType,
				Required:    true,
			},
			"port": {
				Description: "TCP Port",
				Type:        types.Int64Type,
				Required:    true,
			},
			"cooldown_ms": {
				Description: fmt.Sprintf(
					"How many milliseconds to wait before each connection attempt or zero to not wait. Default: %d",
					defaultCooldownMs),
				Type:     types.Int64Type,
				Optional: true,
			},
		},
	}, nil
}

type dataSourcePortWait struct {
	p provider
}

func (d dataSourcePortWaitType) NewDataSource(_ context.Context, p tfsdk.Provider) (tfsdk.DataSource, diag.Diagnostics) {
	return dataSourcePortWait{p: *(p.(*provider))}, nil
}

func (d dataSourcePortWait) tryConnect(state *dataSourcePortWaitType, diags *diag.Diagnostics) {
	address := net.JoinHostPort(state.Host.Value, fmt.Sprintf("%d", state.Port.Value))

	startTime := time.Now()
	endTime := startTime.Add(time.Duration(state.TimeoutSec.Value) * time.Second)
	for {
		var conn net.Conn
		var err error
		if state.TimeoutSec.Value == 0 {
			conn, err = net.Dial("tcp", address)
		} else {
			remainingTime := endTime.Sub(time.Now())
			if remainingTime <= 0 {
				if state.ErrorOnTimeout.Value {
					diags.AddError("connection timeout", "timed out trying to connect to "+address)
					return
				}
				break
			}
			conn, err = net.DialTimeout("tcp", address, remainingTime)
		}
		if err != nil || conn == nil {
			cooldownMs := defaultCooldownMs
			if !state.CooldownMs.Unknown && !state.CooldownMs.Null {
				cooldownMs = state.CooldownMs.Value
			}
			cooldownDur := time.Duration(cooldownMs) * time.Millisecond
			if cooldownMs != 0 {
				if state.TimeoutSec.Value != 0 && time.Now().Add(cooldownDur).After(endTime) {
					// cooldown would go over end time so stop prematurely
					if state.ErrorOnTimeout.Value {
						diags.AddError("connection timeout",
							"prematurely timed out trying to connect to "+address+" (cooldown skipped)")
						return
					}
					break
				}
				time.Sleep(cooldownDur)
			}
			continue
		}
		_ = conn.Close()

		state.Available = types.Bool{Value: true}
		break
	}

	state.ID = types.String{Value: address}
}

func (d dataSourcePortWait) Read(ctx context.Context, req tfsdk.ReadDataSourceRequest, res *tfsdk.ReadDataSourceResponse) {
	state := dataSourcePortWaitType{}
	diags := req.Config.Get(ctx, &state)
	res.Diagnostics.Append(diags...)
	if res.Diagnostics.HasError() {
		return
	}

	d.tryConnect(&state, &res.Diagnostics)

	diags = res.State.Set(ctx, state)
	res.Diagnostics.Append(diags...)
	if res.Diagnostics.HasError() {
		return
	}
}
