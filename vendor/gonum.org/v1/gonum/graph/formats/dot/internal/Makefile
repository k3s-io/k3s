gen: dot.bnf
	gocc $<
	go run paste_copyright.go

debug_lexer: dot.bnf
	gocc -debug_lexer -v -a $<
	go run paste_copyright.go

debug_parser: dot.bnf
	gocc -debug_parser -v -a $<
	go run paste_copyright.go

clean:
	rm -f errors/errors.go
	rm -f lexer/acttab.go
	rm -f lexer/lexer.go
	rm -f lexer/transitiontable.go
	rm -f parser/action.go
	rm -f parser/actiontable.go
	rm -f parser/gototable.go
	rm -f parser/parser.go
	rm -f parser/productionstable.go
	rm -f token/token.go
	rm -f util/litconv.go
	rm -f util/rune.go
	-rmdir --ignore-fail-on-non-empty errors
	-rmdir --ignore-fail-on-non-empty lexer
	-rmdir --ignore-fail-on-non-empty parser
	-rmdir --ignore-fail-on-non-empty token
	-rmdir --ignore-fail-on-non-empty util
	rm -f terminals.txt LR1_conflicts.txt LR1_sets.txt first.txt lexer_sets.txt

.PHONY: gen debug_lexer debug_parser clean
