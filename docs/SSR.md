# Server-Side Rendering (SSR)

`assetmin` supports basic SSR by injecting HTML content from registered components into the `index.html`.

## How it works

When `RegisterComponents` is called, it checks if a component implements `HTMLProvider`.

```go
type HTMLProvider interface {
    RenderHTML() string
}
```

### Security / Access Control

To prevent leaking private data into the public `index.html` (which is served to everyone), `assetmin` strictly checks for public access permissions before injecting HTML.

The component must implement `AccessLevel`:

```go
type AccessLevel interface {
    AllowedRoles(action byte) []byte
}
```

The system checks if `AllowedRoles('r')` contains `'*'` (wildcard/public). If not, the HTML is **ignored**.

## Injection Point

The HTML is injected into the body of `index.html`, effectively pre-rendering the view for the client.
