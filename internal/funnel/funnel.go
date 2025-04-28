package funnel

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"tailscale.com/ipn"
)

type Funnel struct {
	HTTPFunnel *HTTPFunnel
	Client     *TailscaleClient
	Requests   *RequestList
}


// ID returns the unique identifier for the funnel.
func (f *Funnel) ID() string {
	if f.HTTPFunnel == nil {
		return ""
	}
	return f.HTTPFunnel.id
}

// LocalTarget returns the local URL that traffic is ultimately proxied to.
func (f *Funnel) LocalTarget() string {
	if f.HTTPFunnel == nil {
		return ""
	}
	return f.HTTPFunnel.localTarget
}

// RemoteTarget returns the public URL exposed by the Tailscale funnel.
func (f *Funnel) RemoteTarget() string {
	if f.HTTPFunnel == nil {
		return ""
	}
	return f.HTTPFunnel.remoteTarget
}

func (f *Funnel) Name() string {
	if f.HTTPFunnel == nil {
		return ""
	}
	url, err := url.Parse(f.HTTPFunnel.remoteTarget)
	if err != nil {
		return ""
	}
	return strings.Split(url.Hostname(), ".")[0]
}

func (f *Funnel) Destroy() error {
	ctx := context.Background()
	// find the srvConfig, find this record in it, remove it
	srvConfig, err := f.Client.ts.GetServeConfig(ctx)
	if err != nil {
		return err
	}

	url, err := url.Parse(f.HTTPFunnel.remoteTarget)
	if err != nil {
		return err
	}
	port := url.Port()
	if port == "" {
		port = "443"
	}
	hostPort := ipn.HostPort(fmt.Sprintf("%s:%s", url.Hostname(), port))
	portInt, err := hostPort.Port()
	if err != nil {
		return err
	}

	srvConfig.RemoveWebHandler(url.Hostname(), portInt, []string{"/"}, true)

	// now set the serve config
	if err := f.Client.ts.SetServeConfig(ctx, srvConfig); err != nil {
		return err
	}

	// logout the client
	if err := f.Client.ts.Logout(ctx); err != nil {
		return err
	}

	return nil
}

