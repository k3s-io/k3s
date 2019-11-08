package pongo2

// Version string
const Version = "dev"

// Must panics, if a Template couldn't successfully parsed. This is how you
// would use it:
//     var baseTemplate = pongo2.Must(pongo2.FromFile("templates/base.html"))
func Must(tpl *Template, err error) *Template {
	if err != nil {
		panic(err)
	}
	return tpl
}
