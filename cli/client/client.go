package client

import (
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/sensu/sensu-go/cli/client/config"
	resty "gopkg.in/resty.v0"
)

var logger *logrus.Entry

// RestClient wraps resty.Client
type RestClient struct {
	resty  *resty.Client
	config config.Config

	configured   bool
	expiredToken bool
}

func init() {
	logger = logrus.WithFields(logrus.Fields{
		"component": "cli-client",
	})
}

// New builds a new client with defaults
func New(config config.Config) *RestClient {
	restyInst := resty.New()
	client := &RestClient{resty: restyInst, config: config}

	// Standardize redirect policy
	restyInst.SetRedirectPolicy(resty.FlexibleRedirectPolicy(10))

	// JSON
	restyInst.SetHeader("Accept", "application/json")
	restyInst.SetHeader("Content-Type", "application/json")

	// Check that Access-Token has not expired
	restyInst.OnBeforeRequest(func(c *resty.Client, r *resty.Request) error {
		// Pass the organization and environment as query parameters, except when
		// we are creating or updating an object, since we will use the object
		// attributes to determine the org & env
		if r.Method != http.MethodPost && r.Method != http.MethodPut {
			if param := r.QueryParam.Get("env"); param == "" {
				r.SetQueryParam("env", config.Environment())
			}

			if param := r.QueryParam.Get("org"); param == "" {
				r.SetQueryParam("org", config.Organization())
			}
		}

		// Guard against requests that are not sending auth details
		if c.Token == "" || r.UserInfo != nil {
			return nil
		}

		// If the client access token is expired, it means this request is trying to
		// retrieve a new access token and therefore we do not need to do it again
		// otherwise we will have an infinite loop!
		if client.expiredToken {
			return nil
		}

		tokens := config.Tokens()
		expiry := time.Unix(tokens.ExpiresAt, 0)

		// No-op if token has not yet expired
		if hasExpired := expiry.Before(time.Now()); !hasExpired {
			return nil
		}

		if tokens.Refresh == "" {
			return errors.New("configured access token has expired")
		}

		// Mark the token as expired to prevent an infinite loop in this method
		client.expiredToken = true

		// TODO: Move this into it's own file / package
		// Request a new access token from the server
		tokens, err := client.RefreshAccessToken(tokens.Refresh)
		if err != nil {
			return fmt.Errorf(
				"failed to request new refresh token; client returned '%s'",
				err,
			)
		}

		// Write new tokens to disk
		err = config.SaveTokens(tokens)
		if err != nil {
			return fmt.Errorf(
				"failed to update configuration with new refresh token (%s)",
				err,
			)
		}

		// We can now mark the token as valid
		client.expiredToken = false

		c.SetAuthToken(tokens.Access)

		return nil
	})

	// logging
	w := logger.Writer()
	defer func() {
		_ = w.Close()
	}()
	restyInst.SetLogger(w)

	return client
}

// R returns new resty.Request from configured client
func (client *RestClient) R() *resty.Request {
	client.configure()
	request := client.resty.R()

	return request
}

// Reset client so that it reconfigure on next request
func (client *RestClient) Reset() {
	client.configured = false
}

// ClearAuthToken clears the authoization token from the client config
func (client *RestClient) ClearAuthToken() {
	client.configure()
	client.resty.SetAuthToken("")
}

func (client *RestClient) configure() {
	if client.configured {
		return
	}

	restyInst := client.resty
	config := client.config

	// Set URL & access token
	restyInst.SetHostURL(config.APIUrl())

	tokens := config.Tokens()
	if tokens != nil && tokens.Access != "" {
		restyInst.SetAuthToken(tokens.Access)
	}

	client.configured = true
}
