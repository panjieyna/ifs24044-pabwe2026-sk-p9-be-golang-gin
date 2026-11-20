// Package docs menyematkan spesifikasi OpenAPI (Swagger 2.0) ke dalam binary dan
// menyesuaikan host/scheme secara dinamis dengan URL pemanggil.
package docs

import (
	_ "embed"
	"encoding/json"

	"github.com/swaggo/swag"
)

// ApiJSON adalah isi docs/api.json yang di-embed saat kompilasi.
//
//go:embed api.json
var ApiJSON []byte

// SwaggerInfo didaftarkan ke swag agar gin-swagger dapat menemukannya.
var SwaggerInfo = &swag.Spec{
	Version:          "1.0.0",
	Host:             "",
	BasePath:         "/",
	Schemes:          []string{},
	Title:            "Delcom Blog REST API",
	Description:      "REST API Delcom Blog dengan AI Blog Generator dan integrasi Search Engine.",
	InfoInstanceName: "swagger",
	SwaggerTemplate:  string(ApiJSON),
	LeftDelim:        "{{",
	RightDelim:       "}}",
}

// GetSwaggerJSON mengembalikan api.json dengan host dan scheme yang disesuaikan.
func GetSwaggerJSON(host, scheme string) []byte {
	var doc map[string]any
	if err := json.Unmarshal(ApiJSON, &doc); err != nil {
		return ApiJSON
	}
	if host != "" {
		doc["host"] = host
	}
	if scheme != "" {
		doc["schemes"] = []string{scheme}
	}
	updated, _ := json.MarshalIndent(doc, "", "  ")
	return updated
}

func init() {
	swag.Register(SwaggerInfo.InstanceName(), SwaggerInfo)
}
