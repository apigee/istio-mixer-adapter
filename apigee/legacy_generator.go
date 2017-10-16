package apigee

import (
	"encoding/json"
	"fmt"

	"github.com/apigee/istio-mixer-adapter/apigee/config"
	"github.com/apigee/istio-mixer-adapter/apigee/auth"
	"istio.io/mixer/pkg/adapter"
)

const (
	keyParam  = "apikey"
	pathParam = "uripath"
)

func Register(r adapter.Registrar) {
	r.RegisterAttributesGeneratorBuilder(legacyAttrsBuilder{
		adapter.NewDefaultBuilder("apigeelegacy", "apigeelegacy", &config.Params{}),
	})
}

type legacyAttrsBuilder struct {
	adapter.DefaultBuilder
}

func (b legacyAttrsBuilder) ValidateConfig(c adapter.Config) *adapter.ConfigErrors {
	return (&builder{c.(*config.Params), nil}).Validate()
}

func (b legacyAttrsBuilder) BuildAttributesGenerator(env adapter.Env, c adapter.Config) (adapter.AttributesGenerator, error) {

	fmt.Printf("BuildAttributesGenerator: %v\n", c)

	cfg := c.(*config.Params)
	g := &legacyAttrsGenerator{
		env:      env,
		apidBase: cfg.ApidBase,
		orgName:  cfg.OrgName,
	}
	return g, nil
}

type legacyAttrsGenerator struct {
	env      adapter.Env
	apidBase string
	orgName  string
}

func (g *legacyAttrsGenerator) Generate(in map[string]interface{}) (map[string]interface{}, error) {

	fmt.Printf("Generate: %v\n", in)

	out := make(map[string]interface{})

	key := g.getString(in, keyParam)
	if key == "" {
		return out, nil
	}
	path := g.getString(in, pathParam)
	if path == "" {
		return out, nil
	}

	success, fail, err := auth.VerifyAPIKey(g.apidBase, g.orgName, key, path)

	if err != nil {
		return nil, err
	}

	var authResponse []byte
	if fail.ResponseCode != "" {
		authResponse, err = json.Marshal(fail)
		out["apigee.authFailMessage"] = fail.ResponseMessage
	} else {
		authResponse, err = json.Marshal(success)
	}

	if err != nil {
		return nil, err
	}

	fmt.Printf("Auth: %s\n", string(authResponse))

	app := ""
	if len(success.Developer.Apps) > 0 {
		app = success.Developer.Apps[0]
	}

	out["apigee.apiProxy"] = ""
	out["apigee.apiRevision"] = ""
	out["apigee.developerEmail"] = success.Developer.Email
	out["apigee.developerApp"] = app
	out["apigee.accessToken"] = success.ClientId.ClientSecret
	out["apigee.clientID"] = success.ClientId.ClientId
	out["apigee.apiProduct"] = success.ApiProduct.Name

	fmt.Printf("Generated: %v\n", out)

	return out, nil
}

func (g *legacyAttrsGenerator) Close() error {
	return nil
}

func (g *legacyAttrsGenerator) getString(m map[string]interface{}, key string) string {
	v := m[key]
	if v == nil {
		return ""
	}
	switch v.(type) {
	case string:
		return v.(string)
	default:
		return fmt.Sprintf("%v", v)
	}
}
