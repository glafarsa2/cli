package liveshare

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/sourcegraph/jsonrpc2"
)

// Port describes a port exposed by the container.
type Port struct {
	SourcePort                       int    `json:"sourcePort"`
	DestinationPort                  int    `json:"destinationPort"`
	SessionName                      string `json:"sessionName"`
	StreamName                       string `json:"streamName"`
	StreamCondition                  string `json:"streamCondition"`
	BrowseURL                        string `json:"browseUrl"`
	IsPublic                         bool   `json:"isPublic"`
	IsTCPServerConnectionEstablished bool   `json:"isTCPServerConnectionEstablished"`
	HasTLSHandshakePassed            bool   `json:"hasTLSHandshakePassed"`
	Privacy                          string `json:"privacy"`
}

type PortChangeKind string

const (
	PortChangeKindStart  PortChangeKind = "start"
	PortChangeKindUpdate PortChangeKind = "update"
)

type PortUpdate struct {
	Port        int            `json:"port"`
	ChangeKind  PortChangeKind `json:"changeKind"`
	ErrorDetail string         `json:"errorDetail"`
	StatusCode  int            `json:"statusCode"`
}

// startSharing tells the Live Share host to start sharing the specified port from the container.
// The sessionName describes the purpose of the remote port or service.
// It returns an identifier that can be used to open an SSH channel to the remote port.
func (s *Session) startSharing(ctx context.Context, sessionName string, port int) (channelID, error) {
	args := []interface{}{port, sessionName, fmt.Sprintf("http://localhost:%d", port)}
	errc := make(chan error, 1)

	go func() {
		startNotification, err := s.WaitForPortNotification(ctx, port, PortChangeKindStart)
		if err != nil {
			errc <- fmt.Errorf("error while waiting for port notification: %w", err)
			return
		}
		if !startNotification.Success {
			errc <- fmt.Errorf("error while starting port sharing: %s", startNotification.ErrorDetail)
			return
		}
		errc <- nil // success
	}()

	var response Port
	if err := s.rpc.do(ctx, "serverSharing.startSharing", args, &response); err != nil {
		return channelID{}, err
	}

	select {
	case <-ctx.Done():
		return channelID{}, ctx.Err()
	case err := <-errc:
		if err != nil {
			return channelID{}, err
		}
	}

	return channelID{response.StreamName, response.StreamCondition}, nil
}

type PortNotification struct {
	PortUpdate
	Success bool
}

// WaitForPortNotification waits for a port notification to be received. It returns the notification
// or an error if the notification is not received before the context is cancelled or it fails
// to parse the notification.
func (s *Session) WaitForPortNotification(ctx context.Context, port int, notifType PortChangeKind) (*PortNotification, error) {
	notificationUpdate := make(chan PortNotification, 1)
	errc := make(chan error, 1)

	h := func(success bool) func(*jsonrpc2.Conn, *jsonrpc2.Request) {
		return func(conn *jsonrpc2.Conn, req *jsonrpc2.Request) {
			var notification PortNotification
			if err := json.Unmarshal(*req.Params, &notification); err != nil {
				errc <- fmt.Errorf("error unmarshaling notification: %w", err)
				return
			}
			notification.Success = success
			notificationUpdate <- notification
		}
	}
	s.registerRequestHandler("serverSharing.sharingSucceeded", h(true))
	s.registerRequestHandler("serverSharing.sharingFailed", h(false))

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case err := <-errc:
			return nil, err
		case notification := <-notificationUpdate:
			if notification.Port == port && notification.ChangeKind == notifType {
				return &notification, nil
			}
		}
	}
}

// GetSharedServers returns a description of each container port
// shared by a prior call to StartSharing by some client.
func (s *Session) GetSharedServers(ctx context.Context) ([]*Port, error) {
	var response []*Port
	if err := s.rpc.do(ctx, "serverSharing.getSharedServers", []string{}, &response); err != nil {
		return nil, err
	}

	return response, nil
}

// UpdateSharedServerPrivacy controls port permissions and visibility scopes for who can access its URLs
// in the browser.
func (s *Session) UpdateSharedServerPrivacy(ctx context.Context, port int, visibility string) error {
	if err := s.rpc.do(ctx, "serverSharing.updateSharedServerPrivacy", []interface{}{port, visibility}, nil); err != nil {
		return err
	}

	return nil
}
