package pongo2

/* Filters that are provided through github.com/flosch/pongo2-addons:
   ------------------------------------------------------------------

   filesizeformat
   slugify
   timesince
   timeuntil

   Filters that won't be added:
   ----------------------------

   get_static_prefix (reason: web-framework specific)
   pprint (reason: python-specific)
   static (reason: web-framework specific)

   Reconsideration (not implemented yet):
   --------------------------------------

   force_escape (reason: not yet needed since this is the behaviour of pongo2's escape filter)
   safeseq (reason: same reason as `force_escape`)
   unordered_list (python-specific; not sure whether needed or not)
   dictsort (python-specific; maybe one could add a filter to sort a list of structs by a specific field name)
   dictsortreversed (see dictsort)
*/

import (
	"bytes"
	"fmt"
	"math/rand"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/juju/errors"
)

func init() {
	rand.Seed(time.Now().Unix())

	RegisterFilter("escape", filterEscape)
	RegisterFilter("safe", filterSafe)
	RegisterFilter("escapejs", filterEscapejs)

	RegisterFilter("add", filterAdd)
	RegisterFilter("addslashes", filterAddslashes)
	RegisterFilter("capfirst", filterCapfirst)
	RegisterFilter("center", filterCenter)
	RegisterFilter("cut", filterCut)
	RegisterFilter("date", filterDate)
	RegisterFilter("default", filterDefault)
	RegisterFilter("default_if_none", filterDefaultIfNone)
	RegisterFilter("divisibleby", filterDivisibleby)
	RegisterFilter("first", filterFirst)
	RegisterFilter("floatformat", filterFloatformat)
	RegisterFilter("get_digit", filterGetdigit)
	RegisterFilter("iriencode", filterIriencode)
	RegisterFilter("join", filterJoin)
	RegisterFilter("last", filterLast)
	RegisterFilter("length", filterLength)
	RegisterFilter("length_is", filterLengthis)
	RegisterFilter("linebreaks", filterLinebreaks)
	RegisterFilter("linebreaksbr", filterLinebreaksbr)
	RegisterFilter("linenumbers", filterLinenumbers)
	RegisterFilter("ljust", filterLjust)
	RegisterFilter("lower", filterLower)
	RegisterFilter("make_list", filterMakelist)
	RegisterFilter("phone2numeric", filterPhone2numeric)
	RegisterFilter("pluralize", filterPluralize)
	RegisterFilter("random", filterRandom)
	RegisterFilter("removetags", filterRemovetags)
	RegisterFilter("rjust", filterRjust)
	RegisterFilter("slice", filterSlice)
	RegisterFilter("split", filterSplit)
	RegisterFilter("stringformat", filterStringformat)
	RegisterFilter("striptags", filterStriptags)
	RegisterFilter("time", filterDate) // time uses filterDate (same golang-format)
	RegisterFilter("title", filterTitle)
	RegisterFilter("truncatechars", filterTruncatechars)
	RegisterFilter("truncatechars_html", filterTruncatecharsHTML)
	RegisterFilter("truncatewords", filterTruncatewords)
	RegisterFilter("truncatewords_html", filterTruncatewordsHTML)
	RegisterFilter("upper", filterUpper)
	RegisterFilter("urlencode", filterUrlencode)
	RegisterFilter("urlize", filterUrlize)
	RegisterFilter("urlizetrunc", filterUrlizetrunc)
	RegisterFilter("wordcount", filterWordcount)
	RegisterFilter("wordwrap", filterWordwrap)
	RegisterFilter("yesno", filterYesno)

	RegisterFilter("float", filterFloat)     // pongo-specific
	RegisterFilter("integer", filterInteger) // pongo-specific
}

func filterTruncatecharsHelper(s string, newLen int) string {
	runes := []rune(s)
	if newLen < len(runes) {
		if newLen >= 3 {
			return fmt.Sprintf("%s...", string(runes[:newLen-3]))
		}
		// Not enough space for the ellipsis
		return string(runes[:newLen])
	}
	return string(runes)
}

func filterTruncateHTMLHelper(value string, newOutput *bytes.Buffer, cond func() bool, fn func(c rune, s int, idx int) int, finalize func()) {
	vLen := len(value)
	var tagStack []string
	idx := 0

	for idx < vLen && !cond() {
		c, s := utf8.DecodeRuneInString(value[idx:])
		if c == utf8.RuneError {
			idx += s
			continue
		}

		if c == '<' {
			newOutput.WriteRune(c)
			idx += s // consume "<"

			if idx+1 < vLen {
				if value[idx] == '/' {
					// Close tag

					newOutput.WriteString("/")

					tag := ""
					idx++ // consume "/"

					for idx < vLen {
						c2, size2 := utf8.DecodeRuneInString(value[idx:])
						if c2 == utf8.RuneError {
							idx += size2
							continue
						}

						// End of tag found
						if c2 == '>' {
							idx++ // consume ">"
							break
						}
						tag += string(c2)
						idx += size2
					}

					if len(tagStack) > 0 {
						// Ideally, the close tag is TOP of tag stack
						// In malformed HTML, it must not be, so iterate through the stack and remove the tag
						for i := len(tagStack) - 1; i >= 0; i-- {
							if tagStack[i] == tag {
								// Found the tag
								tagStack[i] = tagStack[len(tagStack)-1]
								tagStack = tagStack[:len(tagStack)-1]
								break
							}
						}
					}

					newOutput.WriteString(tag)
					newOutput.WriteString(">")
				} else {
					// Open tag

					tag := ""

					params := false
					for idx < vLen {
						c2, size2 := utf8.DecodeRuneInString(value[idx:])
						if c2 == utf8.RuneError {
							idx += size2
							continue
						}

						newOutput.WriteRune(c2)

						// End of tag found
						if c2 == '>' {
							idx++ // consume ">"
							break
						}

						if !params {
							if c2 == ' ' {
								params = true
							} else {
								tag += string(c2)
							}
						}

						idx += size2
					}

					// Add tag to stack
					tagStack = append(tagStack, tag)
				}
			}
		} else {
			idx = fn(c, s, idx)
		}
	}

	finalize()

	for i := len(tagStack) - 1; i >= 0; i-- {
		tag := tagStack[i]
		// Close everything from the regular tag stack
		newOutput.WriteString(fmt.Sprintf("</%s>", tag))
	}
}

func filterTruncatechars(in *Value, param *Value) (*Value, *Error) {
	s := in.String()
	newLen := param.Integer()
	return AsValue(filterTruncatecharsHelper(s, newLen)), nil
}

func filterTruncatecharsHTML(in *Value, param *Value) (*Value, *Error) {
	value := in.String()
	newLen := max(param.Integer()-3, 0)

	newOutput := bytes.NewBuffer(nil)

	textcounter := 0

	filterTruncateHTMLHelper(value, newOutput, func() bool {
		return textcounter >= newLen
	}, func(c rune, s int, idx int) int {
		textcounter++
		newOutput.WriteRune(c)

		return idx + s
	}, func() {
		if textcounter >= newLen && textcounter < len(value) {
			newOutput.WriteString("...")
		}
	})

	return AsSafeValue(newOutput.String()), nil
}

func filterTruncatewords(in *Value, param *Value) (*Value, *Error) {
	words := strings.Fields(in.String())
	n := param.Integer()
	if n <= 0 {
		return AsValue(""), nil
	}
	nlen := min(len(words), n)
	out := make([]string, 0, nlen)
	for i := 0; i < nlen; i++ {
		out = append(out, words[i])
	}

	if n < len(words) {
		out = append(out, "...")
	}

	return AsValue(strings.Join(out, " ")), nil
}

func filterTruncatewordsHTML(in *Value, param *Value) (*Value, *Error) {
	value := in.String()
	newLen := max(param.Integer(), 0)

	newOutput := bytes.NewBuffer(nil)

	wordcounter := 0

	filterTruncateHTMLHelper(value, newOutput, func() bool {
		return wordcounter >= newLen
	}, func(_ rune, _ int, idx int) int {
		// Get next word
		wordFound := false

		for idx < len(value) {
			c2, size2 := utf8.DecodeRuneInString(value[idx:])
			if c2 == utf8.RuneError {
				idx += size2
				continue
			}

			if c2 == '<' {
				// HTML tag start, don't consume it
				return idx
			}

			newOutput.WriteRune(c2)
			idx += size2

			if c2 == ' ' || c2 == '.' || c2 == ',' || c2 == ';' {
				// Word ends here, stop capturing it now
				break
			} else {
				wordFound = true
			}
		}

		if wordFound {
			wordcounter++
		}

		return idx
	}, func() {
		if wordcounter >= newLen {
			newOutput.WriteString("...")
		}
	})

	return AsSafeValue(newOutput.String()), nil
}

func filterEscape(in *Value, param *Value) (*Value, *Error) {
	output := strings.Replace(in.String(), "&", "&amp;", -1)
	output = strings.Replace(output, ">", "&gt;", -1)
	output = strings.Replace(output, "<", "&lt;", -1)
	output = strings.Replace(output, "\"", "&quot;", -1)
	output = strings.Replace(output, "'", "&#39;", -1)
	return AsValue(output), nil
}

func filterSafe(in *Value, param *Value) (*Value, *Error) {
	return in, nil // nothing to do here, just to keep track of the safe application
}

func filterEscapejs(in *Value, param *Value) (*Value, *Error) {
	sin := in.String()

	var b bytes.Buffer

	idx := 0
	for idx < len(sin) {
		c, size := utf8.DecodeRuneInString(sin[idx:])
		if c == utf8.RuneError {
			idx += size
			continue
		}

		if c == '\\' {
			// Escape seq?
			if idx+1 < len(sin) {
				switch sin[idx+1] {
				case 'r':
					b.WriteString(fmt.Sprintf(`\u%04X`, '\r'))
					idx += 2
					continue
				case 'n':
					b.WriteString(fmt.Sprintf(`\u%04X`, '\n'))
					idx += 2
					continue
					/*case '\'':
						b.WriteString(fmt.Sprintf(`\u%04X`, '\''))
						idx += 2
						continue
					case '"':
						b.WriteString(fmt.Sprintf(`\u%04X`, '"'))
						idx += 2
						continue*/
				}
			}
		}

		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || c == ' ' || c == '/' {
			b.WriteRune(c)
		} else {
			b.WriteString(fmt.Sprintf(`\u%04X`, c))
		}

		idx += size
	}

	return AsValue(b.String()), nil
}

func filterAdd(in *Value, param *Value) (*Value, *Error) {
	if in.IsNumber() && param.IsNumber() {
		if in.IsFloat() || param.IsFloat() {
			return AsValue(in.Float() + param.Float()), nil
		}
		return AsValue(in.Integer() + param.Integer()), nil
	}
	// If in/param is not a number, we're relying on the
	// Value's String() conversion and just add them both together
	return AsValue(in.String() + param.String()), nil
}

func filterAddslashes(in *Value, param *Value) (*Value, *Error) {
	output := strings.Replace(in.String(), "\\", "\\\\", -1)
	output = strings.Replace(output, "\"", "\\\"", -1)
	output = strings.Replace(output, "'", "\\'", -1)
	return AsValue(output), nil
}

func filterCut(in *Value, param *Value) (*Value, *Error) {
	return AsValue(strings.Replace(in.String(), param.String(), "", -1)), nil
}

func filterLength(in *Value, param *Value) (*Value, *Error) {
	return AsValue(in.Len()), nil
}

func filterLengthis(in *Value, param *Value) (*Value, *Error) {
	return AsValue(in.Len() == param.Integer()), nil
}

func filterDefault(in *Value, param *Value) (*Value, *Error) {
	if !in.IsTrue() {
		return param, nil
	}
	return in, nil
}

func filterDefaultIfNone(in *Value, param *Value) (*Value, *Error) {
	if in.IsNil() {
		return param, nil
	}
	return in, nil
}

func filterDivisibleby(in *Value, param *Value) (*Value, *Error) {
	if param.Integer() == 0 {
		return AsValue(false), nil
	}
	return AsValue(in.Integer()%param.Integer() == 0), nil
}

func filterFirst(in *Value, param *Value) (*Value, *Error) {
	if in.CanSlice() && in.Len() > 0 {
		return in.Index(0), nil
	}
	return AsValue(""), nil
}

func filterFloatformat(in *Value, param *Value) (*Value, *Error) {
	val := in.Float()

	decimals := -1
	if !param.IsNil() {
		// Any argument provided?
		decimals = param.Integer()
	}

	// if the argument is not a number (e. g. empty), the default
	// behaviour is trim the result
	trim := !param.IsNumber()

	if decimals <= 0 {
		// argument is negative or zero, so we
		// want the output being trimmed
		decimals = -decimals
		trim = true
	}

	if trim {
		// Remove zeroes
		if float64(int(val)) == val {
			return AsValue(in.Integer()), nil
		}
	}

	return AsValue(strconv.FormatFloat(val, 'f', decimals, 64)), nil
}

func filterGetdigit(in *Value, param *Value) (*Value, *Error) {
	i := param.Integer()
	l := len(in.String()) // do NOT use in.Len() here!
	if i <= 0 || i > l {
		return in, nil
	}
	return AsValue(in.String()[l-i] - 48), nil
}

const filterIRIChars = "/#%[]=:;$&()+,!?*@'~"

func filterIriencode(in *Value, param *Value) (*Value, *Error) {
	var b bytes.Buffer

	sin := in.String()
	for _, r := range sin {
		if strings.IndexRune(filterIRIChars, r) >= 0 {
			b.WriteRune(r)
		} else {
			b.WriteString(url.QueryEscape(string(r)))
		}
	}

	return AsValue(b.String()), nil
}

func filterJoin(in *Value, param *Value) (*Value, *Error) {
	if !in.CanSlice() {
		return in, nil
	}
	sep := param.String()
	sl := make([]string, 0, in.Len())
	for i := 0; i < in.Len(); i++ {
		sl = append(sl, in.Index(i).String())
	}
	return AsValue(strings.Join(sl, sep)), nil
}

func filterLast(in *Value, param *Value) (*Value, *Error) {
	if in.CanSlice() && in.Len() > 0 {
		return in.Index(in.Len() - 1), nil
	}
	return AsValue(""), nil
}

func filterUpper(in *Value, param *Value) (*Value, *Error) {
	return AsValue(strings.ToUpper(in.String())), nil
}

func filterLower(in *Value, param *Value) (*Value, *Error) {
	return AsValue(strings.ToLower(in.String())), nil
}

func filterMakelist(in *Value, param *Value) (*Value, *Error) {
	s := in.String()
	result := make([]string, 0, len(s))
	for _, c := range s {
		result = append(result, string(c))
	}
	return AsValue(result), nil
}

func filterCapfirst(in *Value, param *Value) (*Value, *Error) {
	if in.Len() <= 0 {
		return AsValue(""), nil
	}
	t := in.String()
	r, size := utf8.DecodeRuneInString(t)
	return AsValue(strings.ToUpper(string(r)) + t[size:]), nil
}

func filterCenter(in *Value, param *Value) (*Value, *Error) {
	width := param.Integer()
	slen := in.Len()
	if width <= slen {
		return in, nil
	}

	spaces := width - slen
	left := spaces/2 + spaces%2
	right := spaces / 2

	return AsValue(fmt.Sprintf("%s%s%s", strings.Repeat(" ", left),
		in.String(), strings.Repeat(" ", right))), nil
}

func filterDate(in *Value, param *Value) (*Value, *Error) {
	t, isTime := in.Interface().(time.Time)
	if !isTime {
		return nil, &Error{
			Sender:    "filter:date",
			OrigError: errors.New("filter input argument must be of type 'time.Time'"),
		}
	}
	return AsValue(t.Format(param.String())), nil
}

func filterFloat(in *Value, param *Value) (*Value, *Error) {
	return AsValue(in.Float()), nil
}

func filterInteger(in *Value, param *Value) (*Value, *Error) {
	return AsValue(in.Integer()), nil
}

func filterLinebreaks(in *Value, param *Value) (*Value, *Error) {
	if in.Len() == 0 {
		return in, nil
	}

	var b bytes.Buffer

	// Newline = <br />
	// Double newline = <p>...</p>
	lines := strings.Split(in.String(), "\n")
	lenlines := len(lines)

	opened := false

	for idx, line := range lines {

		if !opened {
			b.WriteString("<p>")
			opened = true
		}

		b.WriteString(line)

		if idx < lenlines-1 && strings.TrimSpace(lines[idx]) != "" {
			// We've not reached the end
			if strings.TrimSpace(lines[idx+1]) == "" {
				// Next line is empty
				if opened {
					b.WriteString("</p>")
					opened = false
				}
			} else {
				b.WriteString("<br />")
			}
		}
	}

	if opened {
		b.WriteString("</p>")
	}

	return AsValue(b.String()), nil
}

func filterSplit(in *Value, param *Value) (*Value, *Error) {
	chunks := strings.Split(in.String(), param.String())

	return AsValue(chunks), nil
}

func filterLinebreaksbr(in *Value, param *Value) (*Value, *Error) {
	return AsValue(strings.Replace(in.String(), "\n", "<br />", -1)), nil
}

func filterLinenumbers(in *Value, param *Value) (*Value, *Error) {
	lines := strings.Split(in.String(), "\n")
	output := make([]string, 0, len(lines))
	for idx, line := range lines {
		output = append(output, fmt.Sprintf("%d. %s", idx+1, line))
	}
	return AsValue(strings.Join(output, "\n")), nil
}

func filterLjust(in *Value, param *Value) (*Value, *Error) {
	times := param.Integer() - in.Len()
	if times < 0 {
		times = 0
	}
	return AsValue(fmt.Sprintf("%s%s", in.String(), strings.Repeat(" ", times))), nil
}

func filterUrlencode(in *Value, param *Value) (*Value, *Error) {
	return AsValue(url.QueryEscape(in.String())), nil
}

// TODO: This regexp could do some work
var filterUrlizeURLRegexp = regexp.MustCompile(`((((http|https)://)|www\.|((^|[ ])[0-9A-Za-z_\-]+(\.com|\.net|\.org|\.info|\.biz|\.de))))(?U:.*)([ ]+|$)`)
var filterUrlizeEmailRegexp = regexp.MustCompile(`(\w+@\w+\.\w{2,4})`)

func filterUrlizeHelper(input string, autoescape bool, trunc int) (string, error) {
	var soutErr error
	sout := filterUrlizeURLRegexp.ReplaceAllStringFunc(input, func(raw_url string) string {
		var prefix string
		var suffix string
		if strings.HasPrefix(raw_url, " ") {
			prefix = " "
		}
		if strings.HasSuffix(raw_url, " ") {
			suffix = " "
		}

		raw_url = strings.TrimSpace(raw_url)

		t, err := ApplyFilter("iriencode", AsValue(raw_url), nil)
		if err != nil {
			soutErr = err
			return ""
		}
		url := t.String()

		if !strings.HasPrefix(url, "http") {
			url = fmt.Sprintf("http://%s", url)
		}

		title := raw_url

		if trunc > 3 && len(title) > trunc {
			title = fmt.Sprintf("%s...", title[:trunc-3])
		}

		if autoescape {
			t, err := ApplyFilter("escape", AsValue(title), nil)
			if err != nil {
				soutErr = err
				return ""
			}
			title = t.String()
		}

		return fmt.Sprintf(`%s<a href="%s" rel="nofollow">%s</a>%s`, prefix, url, title, suffix)
	})
	if soutErr != nil {
		return "", soutErr
	}

	sout = filterUrlizeEmailRegexp.ReplaceAllStringFunc(sout, func(mail string) string {
		title := mail

		if trunc > 3 && len(title) > trunc {
			title = fmt.Sprintf("%s...", title[:trunc-3])
		}

		return fmt.Sprintf(`<a href="mailto:%s">%s</a>`, mail, title)
	})

	return sout, nil
}

func filterUrlize(in *Value, param *Value) (*Value, *Error) {
	autoescape := true
	if param.IsBool() {
		autoescape = param.Bool()
	}

	s, err := filterUrlizeHelper(in.String(), autoescape, -1)
	if err != nil {

	}

	return AsValue(s), nil
}

func filterUrlizetrunc(in *Value, param *Value) (*Value, *Error) {
	s, err := filterUrlizeHelper(in.String(), true, param.Integer())
	if err != nil {
		return nil, &Error{
			Sender:    "filter:urlizetrunc",
			OrigError: errors.New("you cannot pass more than 2 arguments to filter 'pluralize'"),
		}
	}
	return AsValue(s), nil
}

func filterStringformat(in *Value, param *Value) (*Value, *Error) {
	return AsValue(fmt.Sprintf(param.String(), in.Interface())), nil
}

var reStriptags = regexp.MustCompile("<[^>]*?>")

func filterStriptags(in *Value, param *Value) (*Value, *Error) {
	s := in.String()

	// Strip all tags
	s = reStriptags.ReplaceAllString(s, "")

	return AsValue(strings.TrimSpace(s)), nil
}

// https://en.wikipedia.org/wiki/Phoneword
var filterPhone2numericMap = map[string]string{
	"a": "2", "b": "2", "c": "2", "d": "3", "e": "3", "f": "3", "g": "4", "h": "4", "i": "4", "j": "5", "k": "5",
	"l": "5", "m": "6", "n": "6", "o": "6", "p": "7", "q": "7", "r": "7", "s": "7", "t": "8", "u": "8", "v": "8",
	"w": "9", "x": "9", "y": "9", "z": "9",
}

func filterPhone2numeric(in *Value, param *Value) (*Value, *Error) {
	sin := in.String()
	for k, v := range filterPhone2numericMap {
		sin = strings.Replace(sin, k, v, -1)
		sin = strings.Replace(sin, strings.ToUpper(k), v, -1)
	}
	return AsValue(sin), nil
}

func filterPluralize(in *Value, param *Value) (*Value, *Error) {
	if in.IsNumber() {
		// Works only on numbers
		if param.Len() > 0 {
			endings := strings.Split(param.String(), ",")
			if len(endings) > 2 {
				return nil, &Error{
					Sender:    "filter:pluralize",
					OrigError: errors.New("you cannot pass more than 2 arguments to filter 'pluralize'"),
				}
			}
			if len(endings) == 1 {
				// 1 argument
				if in.Integer() != 1 {
					return AsValue(endings[0]), nil
				}
			} else {
				if in.Integer() != 1 {
					// 2 arguments
					return AsValue(endings[1]), nil
				}
				return AsValue(endings[0]), nil
			}
		} else {
			if in.Integer() != 1 {
				// return default 's'
				return AsValue("s"), nil
			}
		}

		return AsValue(""), nil
	}
	return nil, &Error{
		Sender:    "filter:pluralize",
		OrigError: errors.New("filter 'pluralize' does only work on numbers"),
	}
}

func filterRandom(in *Value, param *Value) (*Value, *Error) {
	if !in.CanSlice() || in.Len() <= 0 {
		return in, nil
	}
	i := rand.Intn(in.Len())
	return in.Index(i), nil
}

func filterRemovetags(in *Value, param *Value) (*Value, *Error) {
	s := in.String()
	tags := strings.Split(param.String(), ",")

	// Strip only specific tags
	for _, tag := range tags {
		re := regexp.MustCompile(fmt.Sprintf("</?%s/?>", tag))
		s = re.ReplaceAllString(s, "")
	}

	return AsValue(strings.TrimSpace(s)), nil
}

func filterRjust(in *Value, param *Value) (*Value, *Error) {
	return AsValue(fmt.Sprintf(fmt.Sprintf("%%%ds", param.Integer()), in.String())), nil
}

func filterSlice(in *Value, param *Value) (*Value, *Error) {
	comp := strings.Split(param.String(), ":")
	if len(comp) != 2 {
		return nil, &Error{
			Sender:    "filter:slice",
			OrigError: errors.New("Slice string must have the format 'from:to' [from/to can be omitted, but the ':' is required]"),
		}
	}

	if !in.CanSlice() {
		return in, nil
	}

	from := AsValue(comp[0]).Integer()
	to := in.Len()

	if from > to {
		from = to
	}

	vto := AsValue(comp[1]).Integer()
	if vto >= from && vto <= in.Len() {
		to = vto
	}

	return in.Slice(from, to), nil
}

func filterTitle(in *Value, param *Value) (*Value, *Error) {
	if !in.IsString() {
		return AsValue(""), nil
	}
	return AsValue(strings.Title(strings.ToLower(in.String()))), nil
}

func filterWordcount(in *Value, param *Value) (*Value, *Error) {
	return AsValue(len(strings.Fields(in.String()))), nil
}

func filterWordwrap(in *Value, param *Value) (*Value, *Error) {
	words := strings.Fields(in.String())
	wordsLen := len(words)
	wrapAt := param.Integer()
	if wrapAt <= 0 {
		return in, nil
	}

	linecount := wordsLen/wrapAt + wordsLen%wrapAt
	lines := make([]string, 0, linecount)
	for i := 0; i < linecount; i++ {
		lines = append(lines, strings.Join(words[wrapAt*i:min(wrapAt*(i+1), wordsLen)], " "))
	}
	return AsValue(strings.Join(lines, "\n")), nil
}

func filterYesno(in *Value, param *Value) (*Value, *Error) {
	choices := map[int]string{
		0: "yes",
		1: "no",
		2: "maybe",
	}
	paramString := param.String()
	customChoices := strings.Split(paramString, ",")
	if len(paramString) > 0 {
		if len(customChoices) > 3 {
			return nil, &Error{
				Sender:    "filter:yesno",
				OrigError: errors.Errorf("You cannot pass more than 3 options to the 'yesno'-filter (got: '%s').", paramString),
			}
		}
		if len(customChoices) < 2 {
			return nil, &Error{
				Sender:    "filter:yesno",
				OrigError: errors.Errorf("You must pass either no or at least 2 arguments to the 'yesno'-filter (got: '%s').", paramString),
			}
		}

		// Map to the options now
		choices[0] = customChoices[0]
		choices[1] = customChoices[1]
		if len(customChoices) == 3 {
			choices[2] = customChoices[2]
		}
	}

	// maybe
	if in.IsNil() {
		return AsValue(choices[2]), nil
	}

	// yes
	if in.IsTrue() {
		return AsValue(choices[0]), nil
	}

	// no
	return AsValue(choices[1]), nil
}
