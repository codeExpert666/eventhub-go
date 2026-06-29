# Swagger UI static assets

The CSS and JavaScript files are vendored from `swagger-ui-dist@5.17.14` so
Swagger UI works in offline and restricted enterprise deployments. `index.html`
is maintained in this repository and only references local `/swagger/*` paths.

Update steps:

```sh
tmpdir=$(mktemp -d)
cd "$tmpdir"
npm pack swagger-ui-dist@5.17.14
tar -xf swagger-ui-dist-5.17.14.tgz
cp package/swagger-ui.css package/swagger-ui-bundle.js package/LICENSE package/NOTICE \
  /path/to/eventhub-go/api/openapi/swagger/
```

Keep `index.html` unless the Swagger UI bundle changes its initialization API.

After updating, run:

```sh
go test ./...
make openapi-check
```
