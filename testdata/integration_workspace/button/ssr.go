package button
type Button struct{}
func SSRInstance() *Button { return &Button{} }
func (b *Button) RenderCSS() *Stylesheet { return New(".btn{color:blue;}") }
type Stylesheet string
func (s Stylesheet) String() string { return string(s) }
func New(s string) *Stylesheet { return (*Stylesheet)(&s) }
