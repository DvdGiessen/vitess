/*
Copyright 2019 The Vitess Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package sqlparser

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/stretchr/testify/require"
)

func readable(node Expr) string {
	switch node := node.(type) {
	case *OrExpr:
		return fmt.Sprintf("(%s or %s)", readable(node.Left), readable(node.Right))
	case *AndExpr:
		return fmt.Sprintf("(%s and %s)", readable(node.Left), readable(node.Right))
	case *XorExpr:
		return fmt.Sprintf("(%s xor %s)", readable(node.Left), readable(node.Right))
	case *BinaryExpr:
		return fmt.Sprintf("(%s %s %s)", readable(node.Left), node.Operator.ToString(), readable(node.Right))
	case *IsExpr:
		return fmt.Sprintf("(%s %s)", readable(node.Left), node.Right.ToString())
	default:
		return String(node)
	}
}

func TestAndOrPrecedence(t *testing.T) {
	validSQL := []struct {
		input  string
		output string
	}{{
		input:  "select * from a where a=b and c=d or e=f",
		output: "((a = b and c = d) or e = f)",
	}, {
		input:  "select * from a where a=b or c=d and e=f",
		output: "(a = b or (c = d and e = f))",
	}}
	parser := NewTestParser()
	for _, tcase := range validSQL {
		tree, err := parser.Parse(tcase.input)
		if err != nil {
			t.Error(err)
			continue
		}
		expr := readable(tree.(*Select).Where.Expr)
		if expr != tcase.output {
			t.Errorf("Parse: \n%s, want: \n%s", expr, tcase.output)
		}
	}
}

func TestPlusStarPrecedence(t *testing.T) {
	validSQL := []struct {
		input  string
		output string
	}{{
		input:  "select 1+2*3 from a",
		output: "(1 + (2 * 3))",
	}, {
		input:  "select 1*2+3 from a",
		output: "((1 * 2) + 3)",
	}}
	parser := NewTestParser()
	for _, tcase := range validSQL {
		tree, err := parser.Parse(tcase.input)
		if err != nil {
			t.Error(err)
			continue
		}
		expr := readable(tree.(*Select).SelectExprs.Exprs[0].(*AliasedExpr).Expr)
		if expr != tcase.output {
			t.Errorf("Parse: \n%s, want: \n%s", expr, tcase.output)
		}
	}
}

func TestIsPrecedence(t *testing.T) {
	validSQL := []struct {
		input  string
		output string
	}{{
		input:  "select * from a where a+b is true",
		output: "((a + b) is true)",
	}, {
		input:  "select * from a where a=1 and b=2 is true",
		output: "(a = 1 and (b = 2 is true))",
	}, {
		input:  "select * from a where (a=1 and b=2) is true",
		output: "((a = 1 and b = 2) is true)",
	}}
	parser := NewTestParser()
	for _, tcase := range validSQL {
		tree, err := parser.Parse(tcase.input)
		if err != nil {
			t.Error(err)
			continue
		}
		expr := readable(tree.(*Select).Where.Expr)
		if expr != tcase.output {
			t.Errorf("Parse: \n%s, want: \n%s", expr, tcase.output)
		}
	}
}

func TestParens(t *testing.T) {
	tests := []struct {
		in, expected string
	}{
		{in: "12", expected: "12"},
		{in: "(12)", expected: "12"},
		{in: "((12))", expected: "12"},
		{in: "((true) and (false))", expected: "true and false"},
		{in: "((true) and (false)) and (true)", expected: "true and false and true"},
		{in: "((true) and (false))", expected: "true and false"},
		{in: "a=b and (c=d or e=f)", expected: "a = b and (c = d or e = f)"},
		{in: "(a=b and c=d) or e=f", expected: "a = b and c = d or e = f"},
		{in: "a & (b | c)", expected: "a & (b | c)"},
		{in: "(a & b) | c", expected: "a & b | c"},
		{in: "not (a=b and c=d)", expected: "not (a = b and c = d)"},
		{in: "not (a=b) and c=d", expected: "not a = b and c = d"},
		{in: "(not (a=b)) and c=d", expected: "not a = b and c = d"},
		{in: "-(12)", expected: "-12"},
		{in: "-(12 + 12)", expected: "-(12 + 12)"},
		{in: "(1 > 2) and (1 = b)", expected: "1 > 2 and 1 = b"},
		{in: "(a / b) + c", expected: "a / b + c"},
		{in: "a / (b + c)", expected: "a / (b + c)"},
		{in: "(1,2,3)", expected: "(1, 2, 3)"},
		{in: "(a) between (5) and (7)", expected: "a between 5 and 7"},
		{in: "(a | b) between (5) and (7)", expected: "a | b between 5 and 7"},
		{in: "(a and b) between (5) and (7)", expected: "(a and b) between 5 and 7"},
		{in: "(true is true) is null", expected: "(true is true) is null"},
		{in: "3 * (100 div 3)", expected: "3 * (100 div 3)"},
		{in: "100 div 2 div 2", expected: "100 div 2 div 2"},
		{in: "100 div (2 div 2)", expected: "100 div (2 div 2)"},
		{in: "(100 div 2) div 2", expected: "100 div 2 div 2"},
		{in: "((((((1000))))))", expected: "1000"},
		{in: "100 - (50 + 10)", expected: "100 - (50 + 10)"},
		{in: "100 - 50 + 10", expected: "100 - 50 + 10"},
		{in: "true and (true and true)", expected: "true and (true and true)"},
		{in: "10 - 2 - 1", expected: "10 - 2 - 1"},
		{in: "(10 - 2) - 1", expected: "10 - 2 - 1"},
		{in: "10 - (2 - 1)", expected: "10 - (2 - 1)"},
		{in: "0 <=> (1 and 0)", expected: "0 <=> (1 and 0)"},
		{in: "(~ (1||0)) IS NULL", expected: "~(1 or 0) is null"},
		{in: "1 not like ('a' is null)", expected: "1 not like ('a' is null)"},
		{in: ":vtg1 not like (:vtg2 is null)", expected: ":vtg1 not like (:vtg2 is null)"},
		{in: "a and b member of (c)", expected: "a and b member of (c)"},
		{
			in:       "foo is null and (bar = true or cast('1448364' as unsigned) member of (baz))",
			expected: "foo is null and (bar = true or cast('1448364' as unsigned) member of (baz))",
		},
	}

	parser := NewTestParser()
	for _, tc := range tests {
		t.Run(tc.in, func(t *testing.T) {
			stmt, err := parser.Parse("select " + tc.in)
			require.NoError(t, err)
			out := String(stmt)
			require.Equal(t, "select "+tc.expected+" from dual", out)
		})
	}
}

func TestRandom(t *testing.T) {
	// The purpose of this test is to find discrepancies between Format and parsing. If for example our precedence rules are not consistent between the two, this test should find it.
	// The idea is to generate random queries, and pass them through the parser and then the unparser, and one more time. The result of the first unparse should be the same as the second result.
	g := NewGenerator(5)
	endBy := time.Now().Add(1 * time.Second)

	parser := NewTestParser()
	for !time.Now().After(endBy) {

		// Given a random expression
		randomExpr := g.Expression(ExprGeneratorConfig{})
		inputQ := "select " + String(randomExpr) + " from t"

		// When it's parsed and unparsed
		parsedInput, err := parser.Parse(inputQ)
		require.NoError(t, err, inputQ)

		// Then the unparsing should be the same as the input query
		outputOfParseResult := String(parsedInput)
		require.Equal(t, outputOfParseResult, inputQ)
	}
}

func TestPrecedenceOfMemberOfWithAndWithoutParser(t *testing.T) {
	// This test was used to expose the difference in precedence between the parser and the ast formatter
	expression := "a and b member of (c)"

	// hand coded ast with the expected precedence
	ast1 := &AndExpr{
		Left: NewColName("a"),
		Right: &MemberOfExpr{
			Value:   NewColName("b"),
			JSONArr: NewColName("c"),
		},
	}

	assert.Equal(t, expression, String(ast1))

	// Now let's try it through the parser
	ast2, err := NewTestParser().ParseExpr(expression)
	require.NoError(t, err)

	assert.Equal(t, expression, String(ast2))
}
