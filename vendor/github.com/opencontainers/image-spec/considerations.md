# Extensibility

Implementations that are reading/processing [manifests](manifest.md) or [image indexes](image-index.md) MUST NOT generate an error if they encounter an unknown property.
Instead they MUST ignore unknown properties.

# Canonicalization

* OCI Images are [content-addressable](https://en.wikipedia.org/wiki/Content-addressable_storage). See [descriptors](descriptor.md) for more.
* One benefit of content-addressable storage is easy deduplication.
* Many images might depend on a particular [layer](layer.md), but there will only be one blob in the [store](image-layout.md).
* With a different serialization, that same semantic layer would have a different hash, and if both versions of the layer are referenced there will be two blobs with the same semantic content.
* To allow efficient storage, implementations serializing content for blobs SHOULD use a canonical serialization.
* This increases the chance that different implementations can push the same semantic content to the store without creating redundant blobs.

## JSON

[JSON][] content SHOULD be serialized as [canonical JSON][canonical-json].
Of the [OCI Image Format Specification media types](media-types.md), all the types ending in `+json` contain JSON content.
Implementations:

* [Go][]: [github.com/docker/go][], which claims to implement [canonical JSON][canonical-json] except for Unicode normalization.

[canonical-json]: http://wiki.laptop.org/go/Canonical_JSON
[github.com/docker/go]: https://github.com/docker/go/
[Go]: https://golang.org/
[JSON]: http://json.org/

# EBNF

For field formats described in this specification, we use a limited subset of [Extended Backus-Naur Form][ebnf], similar to that used by the [XML specification][xmlebnf].
Grammars present in the OCI specification are regular and can be converted to a single regular expressions.
However, regular expressions are avoided to limit abiguity between regular expression syntax.
By defining a subset of EBNF used here, the possibility of variation, misunderstanding or ambiguities from linking to a larger specification can be avoided.

Grammars are made up of rules in the following form:

```
symbol ::= expression
```

We can say we have the production identified by symbol if the input is matched by the expression.
Whitespace is completely ignored in rule definitions.

## Expressions

The simplest expression is the literal, surrounded by quotes:

```
literal ::= "matchthis"
```

The above expression defines a symbol, "literal", that matches the exact input of "matchthis".
Character classes are delineated by brackets (`[]`), describing either a set, range or multiple range of characters:

```
set := [abc]
range := [A-Z]
```

The above symbol "set" would match one character of either "a", "b" or "c".
The symbol "range" would match any character, "A" to "Z", inclusive.
Currently, only matching for 7-bit ascii literals and character classes is defined, as that is all that is required by this specification.
Multiple character ranges and explicit characters can be specified in a single character classes, as follows:

```
multipleranges := [a-zA-Z=-]
```

The above matches the characters in the range `A` to `Z`, `a` to `z` and the individual characters `-` and `=`.

Expressions can be made up of one or more expressions, such that one must be followed by the other.
This is known as an implicit concatenation operator.
For example, to satisfy the following rule, both `A` and `B` must be matched to satisfy the rule:

```
symbol ::= A B
```

Each expression must be matched once and only once, `A` followed by `B`.
To support the description of repetition and optional match criteria, the postfix operators `*` and `+` are defined.
`*` indicates that the preceeding expression can be matched zero or more times.
`+` indicates that the preceeding expression must be matched one or more times.
These appear in the following form:

```
zeroormore ::= expression*
oneormore ::= expression+
```

Parentheses are used to group expressions into a larger expression:

```
group ::= (A B)
```

Like simpler expressions above, operators can be applied to groups, as well.
To allow for alternates, we also define the infix operator `|`.

```
oneof ::= A | B
```

The above indicates that the expression should match one of the expressions, `A` or `B`.

## Precedence

The operator precedence is in the following order:

- Terminals (literals and character classes)
- Grouping `()`
- Unary operators `+*`
- Concatenation
- Alternates `|`

The precedence can be better described using grouping to show equivalents.
Concatenation has higher precedence than alernates, such `A B | C D` is equivalent to `(A B) | (C D)`.
Unary operators have higher precedence than alternates and concatenation, such that `A+ | B+` is equivalent to `(A+) | (B+)`.

## Examples

The following combines the previous definitions to match a simple, relative path name, describing the individual components:

```
path      ::= component ("/" component)*
component ::= [a-z]+
```

The production "component" is one or more lowercase letters.
A "path" is then at least one component, possibly followed by zero or more slash-component pairs.
The above can be converted into the following regular expression:

```
[a-z]+(?:/[a-z]+)*
```

[ebnf]: https://en.wikipedia.org/wiki/Extended_Backus%E2%80%93Naur_form
[xmlebnf]: https://www.w3.org/TR/REC-xml/#sec-notation
