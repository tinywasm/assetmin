# Server-Side Rendering (SSR)

`assetmin` supports basic SSR by allowing external orchestrators (like `tinywasm/site`) to inject HTML content into the `index.html`.

## How it works

The orchestrator collects HTML fragments from components and injects them using:

```go
func (am *AssetMin) InjectBodyContent(html string)
```

This method appends the provided HTML to the body of `index.html`, effectively pre-rendering the view for the client.

## Requirements for Injection (SSR Mode)

When using `tinywasm/site` as the orchestrator, the following conditions are enforced for a component's HTML to be automatically injected:

1. **HTMLProvider**: Component implements `RenderHTML() string`
2. **Public Access**: Component implements `AllowedRoles('r')` returning `*`
3. **Module Identifier**: The **first line** of the HTML must contain the word `module` (case-insensitive)

Example of valid HTML (will be injected):
```html
<div class="module-nav">
    <nav>...</nav>
</div>
```

---

> [!NOTE]
> `assetmin` itself no longer iterates over components to extract HTML. This logic has been moved to `tinywasm/site` to better separate asset bundling from application structure.

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
