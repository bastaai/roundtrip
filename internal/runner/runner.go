// Package runner runs requests together with their tests: one at a time for
// the UI (Do), or a whole workspace from the command line (Main).
package runner

import (
	"context"
	"net/http"

	"roundtrip/internal/check"
	"roundtrip/internal/grpcclient"
	"roundtrip/internal/model"
	"roundtrip/internal/sender"
)

// Do sends r with vars resolved, checks its tests against the response
// (reported in Response.Tests) and returns the variables it extracted.
// A failed request is reported in Response.Error, never as a Go error.
// client.Timeout also bounds gRPC calls. protos only matters for gRPC: it is
// the proto config of the request's collection and the workspace directory.
func Do(ctx context.Context, client *http.Client, r model.Request, vars map[string]string, protos grpcclient.Protos) (model.Response, map[string]string) {
	var res model.Response
	if r.Protocol == "grpc" {
		if client.Timeout > 0 {
			var cancel context.CancelFunc
			ctx, cancel = context.WithTimeout(ctx, client.Timeout)
			defer cancel()
		}
		res = grpcclient.Invoke(ctx, r, vars, protos)
	} else {
		res = sender.Send(ctx, client, r, vars)
	}
	var extracted map[string]string
	res.Tests, extracted = check.Run(r, res, vars)
	return res, extracted
}

// Passed reports whether a response counts as a success: there was one and
// every test passed. Without tests, the status decides (HTTP below 400, gRPC OK).
func Passed(r model.Request, res model.Response) bool {
	if res.Error != "" {
		return false
	}
	for _, t := range res.Tests {
		if !t.Passed {
			return false
		}
	}
	return len(r.Tests) > 0 || statusOK(res)
}

func statusOK(res model.Response) bool {
	if res.GRPC {
		return res.Status == 0
	}
	return res.Status < 400
}
