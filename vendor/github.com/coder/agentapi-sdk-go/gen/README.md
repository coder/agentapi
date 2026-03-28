```sh
go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@v2.4.1 \
    -package gen \
    -generate types,client \
    -o api.gen.go \
    openapi-3.0.json
```
