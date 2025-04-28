package funnel

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	stdlog "log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jonson/tsgrok/internal/util"
	"tailscale.com/client/local"
	"tailscale.com/ipn"
	"tailscale.com/ipn/ipnstate"
	"tailscale.com/ipn/store/mem"
	"tailscale.com/tsnet"
)

type TailscaleClient struct {
	ts          *local.Client
	status      *ipnstate.Status
	serveConfig *ipn.ServeConfig
	logger      *stdlog.Logger
}

func (c *TailscaleClient) UpdateStatus() (*ipnstate.Status, error) {
	return c.ts.StatusWithoutPeers(context.Background())
}

func GenerateFunnelID(remoteTarget string, localTarget string) string {

	// for sanity, ensure both remote and local targets have a trailing slash
	if !strings.HasSuffix(remoteTarget, "/") {
		remoteTarget += "/"
	}

	if !strings.HasSuffix(localTarget, "/") {
		localTarget += "/"
	}

	// hash the remote and local targets to create a deterministic id
	id := fmt.Sprintf("remote:%s:local:%s", remoteTarget, localTarget)
	id = fmt.Sprintf("%x", sha256.Sum256([]byte(id)))
	return id
}

// borrowed and adapted from main tailscale client: tailscale/ipn/serve.go
func applyWebServe(sc *ipn.ServeConfig, dnsName string, srvPort uint16, useTLS bool, mount, target string) error {
	h := new(ipn.HTTPHandler)

	// we only support http based targets
	t, err := ipn.ExpandProxyTargetValue(target, []string{"http", "https", "https+insecure"}, "http")
	if err != nil {
		return err
	}
	h.Proxy = t

	if sc.IsTCPForwardingOnPort(srvPort) {
		return errors.New("cannot serve web; already serving TCP")
	}

	sc.SetWebHandler(h, dnsName, srvPort, mount, useTLS)

	return nil
}

// HTTPFunnelOptions holds configuration for creating an HTTP funnel.
type HTTPFunnelOptions struct {
	ID         string // id of the funnel
	LocalPort  uint16 // local port to use
	RemotePort uint16 // remote port to tunnel to use.  one of 443, 8443, 10000
	HTTPS      bool   // local server uses TLS
	Insecure   bool   // ignore TLS certificate errors for local server
	// Mount      string // mount point for the local server, almost always "/" unless you want to serve a subdirectory
	Inspect bool // hijack to local tsgrok server
}

type HTTPFunnel struct {
	id             string
	remoteTarget   string
	internalTarget string
	localTarget    string
	inspect        bool
}

func (c *TailscaleClient) CreateHTTPFunnel(opts HTTPFunnelOptions) (HTTPFunnel, error) {

	if opts.ID == "" {
		opts.ID = uuid.New().String()
	}

	if opts.RemotePort != 443 && opts.RemotePort != 8443 && opts.RemotePort != 10000 {
		return HTTPFunnel{}, fmt.Errorf("invalid remote port %d", opts.RemotePort)
	}

	// todo: get ctx
	status, err := c.ts.StatusWithoutPeers(context.Background())
	if err != nil {
		return HTTPFunnel{}, err
	}
	c.status = status

	sc, err := c.ts.GetServeConfig(context.Background())

	if err != nil {
		return HTTPFunnel{}, err
	}

	if sc == nil {
		sc = new(ipn.ServeConfig)
	}

	// // check if the desired port is already in use
	// funnels, err := c.GetExistingFunnels()
	// if err != nil {
	// 	return err
	// }

	// todo: check if the desired port is already in use
	c.serveConfig = sc

	// look up the host from the config.  remove the trailing '.' if it exists
	host := strings.TrimSuffix(status.Self.DNSName, ".")

	// set the scheme for the local server
	scheme := "http"
	if opts.HTTPS {
		scheme = "https"
	}
	if opts.HTTPS && opts.Insecure {
		scheme = "https+insecure"
	}

	internalPort := uint16(util.DefaultPort)
	internalMount := fmt.Sprintf("/tsgrok/%s", opts.ID)

	remoteTarget := fmt.Sprintf("https://%s:%d", host, opts.RemotePort)
	internalTarget := fmt.Sprintf("%s://localhost:%d%s", scheme, internalPort, internalMount)
	localTarget := fmt.Sprintf("%s://localhost:%d", scheme, opts.LocalPort)

	// Ensure the mount point starts with a slash if not empty
	safeMount := "/"

	// useTLS arg is always true for funnels, they are not allowed to use http
	err = applyWebServe(sc, host, opts.RemotePort, true, safeMount, internalTarget)
	if err != nil {
		return HTTPFunnel{}, err
	}

	sc.SetFunnel(host, opts.RemotePort, true)

	if err := c.ts.SetServeConfig(context.Background(), sc); err != nil {
		return HTTPFunnel{}, err
	}

	// we fire off a request to the new funnel url to trigger certificate generation
	// for the newly created node. i couldn't find a way to trigger it via the Client api
	// but that would be better than this method
	go func() {
		var httpClient = &http.Client{
			Timeout: 30 * time.Second,
		}
		_, err = httpClient.Get(fmt.Sprintf("%s/.well-known/tsgrok/hello", remoteTarget))
		if err != nil {
			c.logger.Printf("Error calling hello: %v\n", err)
		}
	}()

	return HTTPFunnel{
		id:             opts.ID,
		remoteTarget:   remoteTarget,
		internalTarget: internalTarget,
		localTarget:    localTarget,
		inspect:        opts.Inspect,
	}, nil
}

func (c *TailscaleClient) Logout() error {
	return c.ts.Logout(context.Background())
}

func CreateEphemeralFunnel(name string, target string, logger *stdlog.Logger) (Funnel, error) {
	target, err := ipn.ExpandProxyTargetValue(target, []string{"http", "https", "https+insecure"}, "http")
	if err != nil {
		return Funnel{}, err
	}

	targetURL, err := url.Parse(target)
	if err != nil {
		return Funnel{}, err
	}

	localPort := targetURL.Port()
	if localPort == "" {
		return Funnel{}, fmt.Errorf("no port specified in target")
	}
	localPortInt, err := strconv.Atoi(localPort)
	if err != nil {
		return Funnel{}, fmt.Errorf("invalid port %s", localPort)
	}

	memStore, err := mem.New(nil, "tsgrok")
	if err != nil {
		return Funnel{}, err
	}

	ts := &tsnet.Server{
		Hostname:  name,
		Ephemeral: true,
		Store:     memStore,
		AuthKey:   util.GetAuthKey(),
		// Logf:      logger.Printf,
		UserLogf: logger.Printf,
	}

	// we have already checked for auth key, so this would infer bad auth or some other error
	// if it doesn't start up in a reasonable amount of time
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	st, err := ts.Up(ctx)
	if err != nil {
		return Funnel{}, err
	}

	localClient, err := ts.LocalClient()
	if err != nil {
		return Funnel{}, err
	}

	err = ipn.NodeCanFunnel(st.Self)
	if err != nil {
		return Funnel{}, fmt.Errorf("locally created ephmeral node cannot create funnels: %v", err)
	}

	remotePort := uint16(443)
	err = ipn.CheckFunnelPort(remotePort, st.Self)
	if err != nil {
		return Funnel{}, fmt.Errorf("locally created ephmeral node cannot create funnel on port %d: %v", remotePort, err)
	}

	if err := ipn.CheckFunnelAccess(remotePort, st.Self); err != nil {
		return Funnel{}, fmt.Errorf("locally created ephmeral node cannot create funnel on port %d: %v", remotePort, err)
	}

	// ok go create the funnel
	tsClient := TailscaleClient{
		ts: localClient,
	}

	funnelID := uuid.New().String()
	httpFunnel, err := tsClient.CreateHTTPFunnel(HTTPFunnelOptions{
		ID:         funnelID,
		LocalPort:  uint16(localPortInt),
		RemotePort: remotePort,
		HTTPS:      false,
		Inspect:    true,
	})

	if err != nil {
		return Funnel{}, err
	}

	return Funnel{
		HTTPFunnel: &httpFunnel,
		Client:     &tsClient,
		Requests:   &RequestList{maxLength: 100},
	}, nil
}
