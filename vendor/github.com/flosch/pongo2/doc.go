// A Django-syntax like template-engine
//
// Blog posts about pongo2 (including introduction and migration):
// https://www.florian-schlachter.de/?tag=pongo2
//
// Complete documentation on the template language:
// https://docs.djangoproject.com/en/dev/topics/templates/
//
// Try out pongo2 live in the pongo2 playground:
// https://www.florian-schlachter.de/pongo2/
//
// Make sure to read README.md in the repository as well.
//
// A tiny example with template strings:
//
// (Snippet on playground: https://www.florian-schlachter.de/pongo2/?id=1206546277)
//
//     // Compile the template first (i. e. creating the AST)
//     tpl, err := pongo2.FromString("Hello {{ name|capfirst }}!")
//     if err != nil {
//         panic(err)
//     }
//     // Now you can render the template with the given
//     // pongo2.Context how often you want to.
//     out, err := tpl.Execute(pongo2.Context{"name": "fred"})
//     if err != nil {
//         panic(err)
//     }
//     fmt.Println(out) // Output: Hello Fred!
//
package pongo2
