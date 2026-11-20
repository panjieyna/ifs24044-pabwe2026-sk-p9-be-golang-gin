package docs

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetSwaggerJSON_DynamicHostAndScheme(t *testing.T) {
	var doc map[string]any
	assert.NoError(t, json.Unmarshal(GetSwaggerJSON("api.example.test", "https"), &doc))
	assert.Equal(t, "2.0", doc["swagger"])
	assert.Equal(t, "api.example.test", doc["host"])
	assert.Equal(t, []any{"https"}, doc["schemes"])
}

func TestGetSwaggerJSON_KeepsDefaults(t *testing.T) {
	var doc map[string]any
	assert.NoError(t, json.Unmarshal(GetSwaggerJSON("", ""), &doc))
	assert.Equal(t, "", doc["host"])
	assert.Equal(t, []any{"http", "https"}, doc["schemes"])
	assert.Contains(t, doc["paths"], "/api/posts/ai-generate")
}

func TestGetSwaggerJSON_InvalidSpecFallsBack(t *testing.T) {
	original := ApiJSON
	t.Cleanup(func() { ApiJSON = original })
	ApiJSON = []byte("{not json")
	assert.Equal(t, []byte("{not json"), GetSwaggerJSON("h", "http"))
}

func TestSwaggerInfoRegistered(t *testing.T) {
	assert.Equal(t, "Delcom Blog REST API", SwaggerInfo.Title)
	assert.Contains(t, SwaggerInfo.ReadDoc(), "Delcom Blog REST API")
}
