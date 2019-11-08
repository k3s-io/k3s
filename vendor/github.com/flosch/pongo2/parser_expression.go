package pongo2

import (
	"fmt"
	"math"
)

type Expression struct {
	// TODO: Add location token?
	expr1   IEvaluator
	expr2   IEvaluator
	opToken *Token
}

type relationalExpression struct {
	// TODO: Add location token?
	expr1   IEvaluator
	expr2   IEvaluator
	opToken *Token
}

type simpleExpression struct {
	negate       bool
	negativeSign bool
	term1        IEvaluator
	term2        IEvaluator
	opToken      *Token
}

type term struct {
	// TODO: Add location token?
	factor1 IEvaluator
	factor2 IEvaluator
	opToken *Token
}

type power struct {
	// TODO: Add location token?
	power1 IEvaluator
	power2 IEvaluator
}

func (expr *Expression) FilterApplied(name string) bool {
	return expr.expr1.FilterApplied(name) && (expr.expr2 == nil ||
		(expr.expr2 != nil && expr.expr2.FilterApplied(name)))
}

func (expr *relationalExpression) FilterApplied(name string) bool {
	return expr.expr1.FilterApplied(name) && (expr.expr2 == nil ||
		(expr.expr2 != nil && expr.expr2.FilterApplied(name)))
}

func (expr *simpleExpression) FilterApplied(name string) bool {
	return expr.term1.FilterApplied(name) && (expr.term2 == nil ||
		(expr.term2 != nil && expr.term2.FilterApplied(name)))
}

func (expr *term) FilterApplied(name string) bool {
	return expr.factor1.FilterApplied(name) && (expr.factor2 == nil ||
		(expr.factor2 != nil && expr.factor2.FilterApplied(name)))
}

func (expr *power) FilterApplied(name string) bool {
	return expr.power1.FilterApplied(name) && (expr.power2 == nil ||
		(expr.power2 != nil && expr.power2.FilterApplied(name)))
}

func (expr *Expression) GetPositionToken() *Token {
	return expr.expr1.GetPositionToken()
}

func (expr *relationalExpression) GetPositionToken() *Token {
	return expr.expr1.GetPositionToken()
}

func (expr *simpleExpression) GetPositionToken() *Token {
	return expr.term1.GetPositionToken()
}

func (expr *term) GetPositionToken() *Token {
	return expr.factor1.GetPositionToken()
}

func (expr *power) GetPositionToken() *Token {
	return expr.power1.GetPositionToken()
}

func (expr *Expression) Execute(ctx *ExecutionContext, writer TemplateWriter) *Error {
	value, err := expr.Evaluate(ctx)
	if err != nil {
		return err
	}
	writer.WriteString(value.String())
	return nil
}

func (expr *relationalExpression) Execute(ctx *ExecutionContext, writer TemplateWriter) *Error {
	value, err := expr.Evaluate(ctx)
	if err != nil {
		return err
	}
	writer.WriteString(value.String())
	return nil
}

func (expr *simpleExpression) Execute(ctx *ExecutionContext, writer TemplateWriter) *Error {
	value, err := expr.Evaluate(ctx)
	if err != nil {
		return err
	}
	writer.WriteString(value.String())
	return nil
}

func (expr *term) Execute(ctx *ExecutionContext, writer TemplateWriter) *Error {
	value, err := expr.Evaluate(ctx)
	if err != nil {
		return err
	}
	writer.WriteString(value.String())
	return nil
}

func (expr *power) Execute(ctx *ExecutionContext, writer TemplateWriter) *Error {
	value, err := expr.Evaluate(ctx)
	if err != nil {
		return err
	}
	writer.WriteString(value.String())
	return nil
}

func (expr *Expression) Evaluate(ctx *ExecutionContext) (*Value, *Error) {
	v1, err := expr.expr1.Evaluate(ctx)
	if err != nil {
		return nil, err
	}
	if expr.expr2 != nil {
		switch expr.opToken.Val {
		case "and", "&&":
			if !v1.IsTrue() {
				return AsValue(false), nil
			} else {
				v2, err := expr.expr2.Evaluate(ctx)
				if err != nil {
					return nil, err
				}
				return AsValue(v2.IsTrue()), nil
			}
		case "or", "||":
			if v1.IsTrue() {
				return AsValue(true), nil
			} else {
				v2, err := expr.expr2.Evaluate(ctx)
				if err != nil {
					return nil, err
				}
				return AsValue(v2.IsTrue()), nil
			}
		default:
			return nil, ctx.Error(fmt.Sprintf("unimplemented: %s", expr.opToken.Val), expr.opToken)
		}
	} else {
		return v1, nil
	}
}

func (expr *relationalExpression) Evaluate(ctx *ExecutionContext) (*Value, *Error) {
	v1, err := expr.expr1.Evaluate(ctx)
	if err != nil {
		return nil, err
	}
	if expr.expr2 != nil {
		v2, err := expr.expr2.Evaluate(ctx)
		if err != nil {
			return nil, err
		}
		switch expr.opToken.Val {
		case "<=":
			if v1.IsFloat() || v2.IsFloat() {
				return AsValue(v1.Float() <= v2.Float()), nil
			}
			return AsValue(v1.Integer() <= v2.Integer()), nil
		case ">=":
			if v1.IsFloat() || v2.IsFloat() {
				return AsValue(v1.Float() >= v2.Float()), nil
			}
			return AsValue(v1.Integer() >= v2.Integer()), nil
		case "==":
			return AsValue(v1.EqualValueTo(v2)), nil
		case ">":
			if v1.IsFloat() || v2.IsFloat() {
				return AsValue(v1.Float() > v2.Float()), nil
			}
			return AsValue(v1.Integer() > v2.Integer()), nil
		case "<":
			if v1.IsFloat() || v2.IsFloat() {
				return AsValue(v1.Float() < v2.Float()), nil
			}
			return AsValue(v1.Integer() < v2.Integer()), nil
		case "!=", "<>":
			return AsValue(!v1.EqualValueTo(v2)), nil
		case "in":
			return AsValue(v2.Contains(v1)), nil
		default:
			return nil, ctx.Error(fmt.Sprintf("unimplemented: %s", expr.opToken.Val), expr.opToken)
		}
	} else {
		return v1, nil
	}
}

func (expr *simpleExpression) Evaluate(ctx *ExecutionContext) (*Value, *Error) {
	t1, err := expr.term1.Evaluate(ctx)
	if err != nil {
		return nil, err
	}
	result := t1

	if expr.negate {
		result = result.Negate()
	}

	if expr.negativeSign {
		if result.IsNumber() {
			switch {
			case result.IsFloat():
				result = AsValue(-1 * result.Float())
			case result.IsInteger():
				result = AsValue(-1 * result.Integer())
			default:
				return nil, ctx.Error("Operation between a number and a non-(float/integer) is not possible", nil)
			}
		} else {
			return nil, ctx.Error("Negative sign on a non-number expression", expr.GetPositionToken())
		}
	}

	if expr.term2 != nil {
		t2, err := expr.term2.Evaluate(ctx)
		if err != nil {
			return nil, err
		}
		switch expr.opToken.Val {
		case "+":
			if result.IsFloat() || t2.IsFloat() {
				// Result will be a float
				return AsValue(result.Float() + t2.Float()), nil
			}
			// Result will be an integer
			return AsValue(result.Integer() + t2.Integer()), nil
		case "-":
			if result.IsFloat() || t2.IsFloat() {
				// Result will be a float
				return AsValue(result.Float() - t2.Float()), nil
			}
			// Result will be an integer
			return AsValue(result.Integer() - t2.Integer()), nil
		default:
			return nil, ctx.Error("Unimplemented", expr.GetPositionToken())
		}
	}

	return result, nil
}

func (expr *term) Evaluate(ctx *ExecutionContext) (*Value, *Error) {
	f1, err := expr.factor1.Evaluate(ctx)
	if err != nil {
		return nil, err
	}
	if expr.factor2 != nil {
		f2, err := expr.factor2.Evaluate(ctx)
		if err != nil {
			return nil, err
		}
		switch expr.opToken.Val {
		case "*":
			if f1.IsFloat() || f2.IsFloat() {
				// Result will be float
				return AsValue(f1.Float() * f2.Float()), nil
			}
			// Result will be int
			return AsValue(f1.Integer() * f2.Integer()), nil
		case "/":
			if f1.IsFloat() || f2.IsFloat() {
				// Result will be float
				return AsValue(f1.Float() / f2.Float()), nil
			}
			// Result will be int
			return AsValue(f1.Integer() / f2.Integer()), nil
		case "%":
			// Result will be int
			return AsValue(f1.Integer() % f2.Integer()), nil
		default:
			return nil, ctx.Error("unimplemented", expr.opToken)
		}
	} else {
		return f1, nil
	}
}

func (expr *power) Evaluate(ctx *ExecutionContext) (*Value, *Error) {
	p1, err := expr.power1.Evaluate(ctx)
	if err != nil {
		return nil, err
	}
	if expr.power2 != nil {
		p2, err := expr.power2.Evaluate(ctx)
		if err != nil {
			return nil, err
		}
		return AsValue(math.Pow(p1.Float(), p2.Float())), nil
	}
	return p1, nil
}

func (p *Parser) parseFactor() (IEvaluator, *Error) {
	if p.Match(TokenSymbol, "(") != nil {
		expr, err := p.ParseExpression()
		if err != nil {
			return nil, err
		}
		if p.Match(TokenSymbol, ")") == nil {
			return nil, p.Error("Closing bracket expected after expression", nil)
		}
		return expr, nil
	}

	return p.parseVariableOrLiteralWithFilter()
}

func (p *Parser) parsePower() (IEvaluator, *Error) {
	pw := new(power)

	power1, err := p.parseFactor()
	if err != nil {
		return nil, err
	}
	pw.power1 = power1

	if p.Match(TokenSymbol, "^") != nil {
		power2, err := p.parsePower()
		if err != nil {
			return nil, err
		}
		pw.power2 = power2
	}

	if pw.power2 == nil {
		// Shortcut for faster evaluation
		return pw.power1, nil
	}

	return pw, nil
}

func (p *Parser) parseTerm() (IEvaluator, *Error) {
	returnTerm := new(term)

	factor1, err := p.parsePower()
	if err != nil {
		return nil, err
	}
	returnTerm.factor1 = factor1

	for p.PeekOne(TokenSymbol, "*", "/", "%") != nil {
		if returnTerm.opToken != nil {
			// Create new sub-term
			returnTerm = &term{
				factor1: returnTerm,
			}
		}

		op := p.Current()
		p.Consume()

		factor2, err := p.parsePower()
		if err != nil {
			return nil, err
		}

		returnTerm.opToken = op
		returnTerm.factor2 = factor2
	}

	if returnTerm.opToken == nil {
		// Shortcut for faster evaluation
		return returnTerm.factor1, nil
	}

	return returnTerm, nil
}

func (p *Parser) parseSimpleExpression() (IEvaluator, *Error) {
	expr := new(simpleExpression)

	if sign := p.MatchOne(TokenSymbol, "+", "-"); sign != nil {
		if sign.Val == "-" {
			expr.negativeSign = true
		}
	}

	if p.Match(TokenSymbol, "!") != nil || p.Match(TokenKeyword, "not") != nil {
		expr.negate = true
	}

	term1, err := p.parseTerm()
	if err != nil {
		return nil, err
	}
	expr.term1 = term1

	for p.PeekOne(TokenSymbol, "+", "-") != nil {
		if expr.opToken != nil {
			// New sub expr
			expr = &simpleExpression{
				term1: expr,
			}
		}

		op := p.Current()
		p.Consume()

		term2, err := p.parseTerm()
		if err != nil {
			return nil, err
		}

		expr.term2 = term2
		expr.opToken = op
	}

	if expr.negate == false && expr.negativeSign == false && expr.term2 == nil {
		// Shortcut for faster evaluation
		return expr.term1, nil
	}

	return expr, nil
}

func (p *Parser) parseRelationalExpression() (IEvaluator, *Error) {
	expr1, err := p.parseSimpleExpression()
	if err != nil {
		return nil, err
	}

	expr := &relationalExpression{
		expr1: expr1,
	}

	if t := p.MatchOne(TokenSymbol, "==", "<=", ">=", "!=", "<>", ">", "<"); t != nil {
		expr2, err := p.parseRelationalExpression()
		if err != nil {
			return nil, err
		}
		expr.opToken = t
		expr.expr2 = expr2
	} else if t := p.MatchOne(TokenKeyword, "in"); t != nil {
		expr2, err := p.parseSimpleExpression()
		if err != nil {
			return nil, err
		}
		expr.opToken = t
		expr.expr2 = expr2
	}

	if expr.expr2 == nil {
		// Shortcut for faster evaluation
		return expr.expr1, nil
	}

	return expr, nil
}

func (p *Parser) ParseExpression() (IEvaluator, *Error) {
	rexpr1, err := p.parseRelationalExpression()
	if err != nil {
		return nil, err
	}

	exp := &Expression{
		expr1: rexpr1,
	}

	if p.PeekOne(TokenSymbol, "&&", "||") != nil || p.PeekOne(TokenKeyword, "and", "or") != nil {
		op := p.Current()
		p.Consume()
		expr2, err := p.ParseExpression()
		if err != nil {
			return nil, err
		}
		exp.expr2 = expr2
		exp.opToken = op
	}

	if exp.expr2 == nil {
		// Shortcut for faster evaluation
		return exp.expr1, nil
	}

	return exp, nil
}
