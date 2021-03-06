package rethinkgo

// Based off of RethinkDB's javascript test.js
// https://github.com/rethinkdb/rethinkdb/blob/next/drivers/javascript/rethinkdb/test.js

import (
	"encoding/json"
	"fmt"
	. "launchpad.net/gocheck"
	"testing"
)

// Global expressions used in tests
var arr = Expr(1, 2, 3, 4, 5, 6)
var tobj = Expr(Map{"a": 1, "b": 2, "c": 3})
var tbl = Table("table1")
var tbl2 = Table("table2")
var tbl3 = Table("table3")
var tbl4 = Table("table4")
var gobj = Expr(List{
	Map{"g1": 1, "g2": 1, "num": 0},
	Map{"g1": 1, "g2": 2, "num": 5},
	Map{"g1": 1, "g2": 2, "num": 10},
	Map{"g1": 2, "g2": 3, "num": 0},
	Map{"g1": 2, "g2": 3, "num": 100},
})
var j1 = Table("joins1")
var j2 = Table("joins2")
var j3 = Table("joins3")
var docs = []Map{
	Map{"id": 0},
	Map{"id": 1},
	Map{"id": 2},
	Map{"id": 3},
	Map{"id": 4},
	Map{"id": 5},
	Map{"id": 6},
	Map{"id": 7},
	Map{"id": 8},
	Map{"id": 9},
}
var session *Session

// Hook up gocheck into the gotest runner.
func Test(t *testing.T) { TestingT(t) }

type RethinkSuite struct{}

func (s *RethinkSuite) SetUpSuite(c *C) {
	SetDebug(true)
	var err error
	session, err = Connect("localhost:28015", "test")
	c.Assert(err, IsNil)

	resetDatabase(c)
}

func (s *RethinkSuite) TearDownSuite(c *C) {
	session.Close()
}

func resetDatabase(c *C) {
	// Drop the test database, then re-create it with some test data
	DbDrop("test").Run(session)
	err := DbCreate("test").Run(session).Err()
	c.Assert(err, IsNil)

	err = Db("test").TableCreate("table1").Run(session).Err()
	c.Assert(err, IsNil)

	pair := ExpectPair{tbl.Insert(Map{"id": 0, "num": 20}), Map{"inserted": 1, "errors": 0}}
	runSimpleQuery(c, pair)

	var others []Map
	for i := 1; i < 10; i++ {
		others = append(others, Map{"id": i, "num": 20 - i})
	}
	pair = ExpectPair{tbl.Insert(others), Map{"inserted": 9, "errors": 0}}
	runSimpleQuery(c, pair)

	err = Db("test").TableCreate("table2").Run(session).Err()
	c.Assert(err, IsNil)

	pair = ExpectPair{tbl2.Insert(List{
		Map{"id": 20, "name": "bob"},
		Map{"id": 19, "name": "tom"},
		Map{"id": 18, "name": "joe"},
	}), Map{"inserted": 3, "errors": 0}}
	runSimpleQuery(c, pair)

	// det
	err = Db("test").TableCreate("table3").Run(session).Err()
	c.Assert(err, IsNil)

	err = tbl3.Insert(docs).Run(session).Err()
	c.Assert(err, IsNil)

	err = Db("test").TableCreate("table4").Run(session).Err()
	c.Assert(err, IsNil)

	// joins tables
	s1 := List{
		Map{"id": 0, "name": "bob"},
		Map{"id": 1, "name": "tom"},
		Map{"id": 2, "name": "joe"},
	}
	s2 := List{
		Map{"id": 0, "title": "goof"},
		Map{"id": 2, "title": "lmoe"},
	}
	s3 := List{
		Map{"it": 0, "title": "goof"},
		Map{"it": 2, "title": "lmoe"},
	}

	Db("test").TableCreate("joins1").Run(session)
	j1.Insert(s1).Run(session)
	Db("test").TableCreate("joins2").Run(session)
	j2.Insert(s2).Run(session)
	spec := TableSpec{Name: "joins3", PrimaryKey: "it"}
	Db("test").TableCreateSpec(spec).Run(session)
	j3.Insert(s3).Run(session)
}

var _ = Suite(&RethinkSuite{})

type jsonChecker struct {
	info *CheckerInfo
}

func (j jsonChecker) Info() *CheckerInfo {
	return j.info
}

func (j jsonChecker) Check(params []interface{}, names []string) (result bool, error string) {
	var jsonParams []interface{}
	for _, param := range params {
		jsonParam, err := json.Marshal(param)
		if err != nil {
			return false, err.Error()
		}
		jsonParams = append(jsonParams, jsonParam)
	}
	return DeepEquals.Check(jsonParams, names)
}

// JsonEquals compares two interface{} objects by converting them to JSON and
// seeing if the strings match
var JsonEquals = &jsonChecker{
	&CheckerInfo{Name: "JsonEquals", Params: []string{"obtained", "expected"}},
}

type ExpectPair struct {
	query    Query
	expected interface{}
}

type MatchMap map[string]interface{}

// Used to indicate that we expect an error from the server
type ErrorResponse struct{}

func runSimpleQuery(c *C, pair ExpectPair) {
	var result interface{}
	fmt.Println("query:", pair.query)
	err := pair.query.Run(session).One(&result)
	fmt.Printf("result: %v %T\n", result, result)
	_, ok := pair.expected.(ErrorResponse)
	if ok {
		c.Assert(err, NotNil)
		return
	} else {
		c.Assert(err, IsNil)
	}

	// when reading in a number into an interface{}, the json library seems to
	// choose float64 as the type to use
	// since c.Assert() compares the types directly, we need to make sure to pass
	// it a float64 if we have a number
	switch v := pair.expected.(type) {
	case int:
		c.Assert(result, Equals, float64(v))
	case Map, List:
		// Even if v is converted with toObject(), the maps don't seem to compare
		// correctly with gocheck, and the gocheck api docs don't mention maps, so
		// just convert to a []byte with json, then compare the bytes
		c.Assert(result, JsonEquals, pair.expected)
	case MatchMap:
		// In some cases we want to match against a map, but only against those keys
		// that appear in the map, not against all keys in the result, the MatchMap
		// type does this.
		resultMap := result.(map[string]interface{})
		filteredResult := map[string]interface{}{}
		for key, _ := range v {
			filteredResult[key] = resultMap[key]
		}
		c.Assert(filteredResult, JsonEquals, pair.expected)
	default:
		c.Assert(result, Equals, pair.expected)
	}
}

func runStreamQuery(c *C, pair ExpectPair) {
	var result []interface{}
	err := pair.query.Run(session).Collect(&result)
	fmt.Printf("result: %v %T\n", result, result)
	c.Assert(err, IsNil)
	c.Assert(result, JsonEquals, pair.expected)
}

var testSimpleGroups = map[string][]ExpectPair{
	"basic": {
		{Expr(1), 1},
		{Expr(true), true},
		{Expr("bob"), "bob"},
		{Expr(nil), nil},
	},
	"arith": {
		{Expr(1).Add(2), 3},
		{Expr(1).Sub(2), -1},
		{Expr(5).Mul(8), 40},
		{Expr(8).Div(2), 4},
		{Expr(7).Mod(2), 1},
	},
	"compare": {
		{Expr(1).Eq(1), true},
		{Expr(1).Eq(2), false},
		{Expr(1).Lt(2), true},
		{Expr(8).Lt(-4), false},
		{Expr(8).Le(8), true},
		{Expr(8).Gt(7), true},
		{Expr(8).Gt(8), false},
		{Expr(8).Ge(8), true},
	},
	"bool": {
		{Expr(true).Not(), false},
		{Expr(true).And(true), true},
		{Expr(true).And(false), false},
		{Expr(true).Or(false), true},
		// DeMorgan's
		{Expr(true).And(false).Eq(Expr(true).Not().Or(Expr(false).Not()).Not()), true},
	},
	"slices": {
		{arr.Nth(0), 1},
		{arr.Count(), 6},
		{arr.Limit(5).Count(), 5},
		{arr.Skip(4).Count(), 2},
		{arr.Skip(4).Nth(0), 5},
		{arr.Slice(1, 4).Count(), 3},
		{arr.Nth(2), 3},
	},
	"append": {
		{arr.Append(7).Nth(6), 7},
	},
	"merge": {
		{Expr(Map{"a": 1}).Merge(Map{"b": 2}), Map{"a": 1, "b": 2}},
	},
	"if": {
		{Branch(true, 1, 2), 1},
		{Branch(false, 1, 2), 2},
		{Branch(Expr(2).Mul(8).Ge(Expr(30).Div(2)), Expr(8).Div(2), Expr(9).Div(3)), 4},
	},
	"let": {
		{Let(Map{"a": 1}, LetVar("a")), 1},
		{Let(Map{"a": 1, "b": 2}, LetVar("a").Add(LetVar("b"))), 3},
	},
	"distinct": {
		{Expr(1, 1, 2, 3, 3, 3, 3).Distinct(), List{1, 2, 3}},
	},
	"map": {
		{arr.Map(func(a Exp) Exp {
			return a.Add(1)
		}).Nth(2),
			4,
		},
	},
	"reduce": {
		{arr.Reduce(0, func(a, b Exp) Exp {
			return a.Add(b)
		}),
			21,
		},
	},
	"filter": {
		{arr.Filter(func(val Exp) Exp {
			return val.Lt(3)
		}).Count(),
			2,
		},
	},
	"contains": {
		{tobj.Contains("a"), true},
		{tobj.Contains("d"), false},
		{tobj.Contains("a", "c"), true},
		{tobj.Contains("a", "d"), false},
	},
	"getattr": {
		{tobj.Attr("a"), 1},
		{tobj.Attr("b"), 2},
		{tobj.Attr("c"), 3},
	},
	"pickattrs": {
		{tobj.Pick("a"), Map{"a": 1}},
		{tobj.Pick("a", "b"), Map{"a": 1, "b": 2}},
	},
	"unpick": {
		{tobj.Unpick("a"), Map{"b": 2, "c": 3}},
		{tobj.Unpick("a", "b"), Map{"c": 3}},
	},
	"r": {
		{Let(Map{"a": Map{"b": 1}}, LetVar("a").Attr("b")), 1},
	},
	"orderby": {
		{tbl.OrderBy("num").Nth(2), Map{"id": 7, "num": 13}},
		{tbl.OrderBy("num").Nth(2).Pick("num"), Map{"num": 13}},
		{tbl.OrderBy(Asc("num")).Nth(2), Map{"id": 7, "num": 13}},
		{tbl.OrderBy(Asc("num")).Nth(2).Pick("num"), Map{"num": 13}},
		{tbl.OrderBy(Desc("num")).Nth(2), Map{"id": 2, "num": 18}},
		{tbl.OrderBy(Desc("num")).Nth(2).Pick("num"), Map{"num": 18}},
	},
	"pluck": {
		{tbl.OrderBy("num").Pluck("num").Nth(0), Map{"num": 11}},
	},
	"without": {
		{tbl.OrderBy("num").Without("num").Nth(0), Map{"id": 9}},
	},
	"union": {
		{Expr(1, 2, 3).Union(List{4, 5, 6}), List{1, 2, 3, 4, 5, 6}},
		{tbl.Union(tbl).Count().Eq(tbl.Count().Mul(2)), true},
	},
	"tablefilter": {
		{tbl.Filter(func(row Exp) Exp {
			return row.Attr("num").Gt(16)
		}).Count(),
			4,
		},
		{tbl.Filter(Row.Attr("num").Gt(16)).Count(), 4},
		{tbl.Filter(Map{"num": 16}).Nth(0), Map{"id": 4, "num": 16}},
		{tbl.Filter(Map{"num": Expr(20).Sub(Row.Attr("id"))}).Count(), 10},
	},
	"tablemap": {
		{tbl.OrderBy("num").Map(Row.Attr("num")).Nth(2), 13},
	},
	"tablereduce": {
		{tbl.Map(Row.Attr("num")).Reduce(0, func(a, b Exp) Exp { return b.Add(a) }), 155},
	},
	"tablechain": {
		{tbl.Filter(func(row Exp) Exp {
			return Row.Attr("num").Gt(16)
		}).Count(),
			4,
		},

		{tbl.Map(func(row Exp) Exp {
			return Row.Attr("num").Add(2)
		}).Filter(func(val Exp) Exp {
			return val.Gt(16)
		}).Count(),
			6,
		},

		{tbl.Filter(func(row Exp) Exp {
			return Row.Attr("num").Gt(16)
		}).Map(func(row Exp) Exp {
			return row.Attr("num").Mul(4)
		}).Reduce(0, func(acc, val Exp) Exp {
			return acc.Add(val)
		}),
			296,
		},
	},
	"between": {
		{tbl.BetweenIds(2, 3).Count(), 2},
		{tbl.Between("id", 2, 3).OrderBy("id").Nth(0), Map{"id": 2, "num": 18}},
	},
	"groupedmapreduce": {
		{tbl.GroupedMapReduce(
			func(row Exp) Exp {
				return Branch(row.Attr("id").Lt(5), 0, 1)
			},
			func(row Exp) Exp {
				return row.Attr("num")
			},
			0,
			func(acc, num Exp) Exp {
				return acc.Add(num)
			},
		),
			List{
				Map{"group": 0, "reduction": 90},
				Map{"group": 1, "reduction": 65},
			},
		},
	},
	"groupby": {
		{gobj.GroupBy("g1", Avg("num")),
			List{
				Map{"group": 1, "reduction": 5},
				Map{"group": 2, "reduction": 50},
			},
		},
		{gobj.GroupBy("g1", Count()),
			List{
				Map{"group": 1, "reduction": 3},
				Map{"group": 2, "reduction": 2},
			},
		},
		{gobj.GroupBy("g1", Sum("num")),
			List{
				Map{"group": 1, "reduction": 15},
				Map{"group": 2, "reduction": 100},
			},
		},
		{gobj.GroupBy([]string{"g1", "g2"}, Avg("num")),
			List{
				Map{"group": List{1, 1}, "reduction": 0},
				Map{"group": List{1, 2}, "reduction": 7.5},
				Map{"group": List{2, 3}, "reduction": 50},
			},
		},
	},
	"concatmap": {
		{tbl.ConcatMap(List{1, 2}).Count(), 20},
	},
	"update": {
		{tbl.Filter(func(row Exp) Exp {
			return row.Attr("id").Ge(5)
		}).Update(func(a Exp) Exp {
			return a.Merge(Map{"updated": true})
		}),
			Map{
				"errors":  0,
				"skipped": 0,
				"updated": 5,
			},
		},
		{tbl.Filter(func(row Exp) Exp {
			return row.Attr("id").Lt(5)
		}).Update(func(a Exp) Exp {
			return a.Merge(Map{"updated": true})
		}),
			Map{
				"errors":  0,
				"skipped": 0,
				"updated": 5,
			},
		},
		{tbl.Filter(func(row Exp) Exp {
			return row.Attr("updated").Eq(true)
		}).Count(), 10},
	},
	"pointupdate": {
		{tbl.GetById(0).Update(func(row Exp) Exp {
			return row.Merge(Map{"pointupdated": true})
		}),
			Map{
				"errors":  0,
				"skipped": 0,
				"updated": 1,
			},
		},
		{tbl.GetById(0).Attr("pointupdated"), true},
	},
	"replace": {
		{tbl.Replace(func(row Exp) Exp {
			return row.Pick("id").Merge(Map{"mutated": true})
		}),
			Map{
				"deleted":  0,
				"errors":   0,
				"inserted": 0,
				"modified": 10,
			},
		},
		{tbl.Filter(func(row Exp) Exp {
			return row.Attr("mutated").Eq(true)
		}).Count(),
			10,
		},
	},
	"pointreplace": {
		{tbl.GetById(0).Replace(func(row Exp) Exp {
			return row.Pick("id").Merge(Map{"pointmutated": true})
		}),
			Map{
				"deleted":  0,
				"errors":   0,
				"inserted": 0,
				"modified": 1,
			},
		},
		{tbl.GetById(0).Attr("pointmutated"), true},
	},
	"det": {
		{tbl3.Update(func(row Exp) interface{} {
			return Map{"count": Js(`0`)}
		}),
			MatchMap{"errors": 10},
		},
		{tbl3.Update(func(row Exp) interface{} {
			return Map{"count": 0}
		}),
			MatchMap{"updated": 10},
		},
		{tbl3.Replace(func(row Exp) interface{} {
			return tbl3.GetById(row.Attr("id"))
		}),
			MatchMap{"errors": 10},
		},
		{tbl3.Replace(func(row Exp) interface{} {
			return row
		}),
			MatchMap{},
		},
		{tbl3.Update(Map{"count": tbl3.Map(func(x Exp) interface{} {
			return x.Attr("count")
		}).Reduce(0, func(a, b Exp) Exp {
			return a.Add(b)
		})}),
			MatchMap{"errors": 10},
		},
		{tbl3.Update(Map{"count": Expr(docs).Map(func(x Exp) interface{} {
			return x.Attr("id")
		}).Reduce(0, func(a, b Exp) Exp {
			return a.Add(b)
		})}),
			MatchMap{"updated": 10},
		},
	},
	"nonatomic": {
		{tbl3.Update(func(row Exp) interface{} {
			return Map{"count": 0}
		}),
			MatchMap{"updated": 10},
		},
		{tbl3.Update(func(row Exp) interface{} {
			return Map{"x": Js(`1`)}
		}),
			MatchMap{"errors": 10},
		},
		{tbl3.Update(func(row Exp) interface{} {
			return Map{"x": Js(`1`)}
		}).Atomic(false),
			MatchMap{"updated": 10},
		},
		{tbl3.Map(func(row Exp) interface{} {
			return row.Attr("x")
		}).Reduce(0, func(a, b Exp) Exp {
			return a.Add(b)
		}),
			10,
		},
		{tbl3.GetById(0).Update(func(row Exp) interface{} {
			return Map{"x": Js(`1`)}
		}),
			ErrorResponse{},
		},
		{tbl3.GetById(0).Update(func(row Exp) interface{} {
			return Map{"x": Js(`2`)}
		}).Atomic(false),
			MatchMap{"updated": 1},
		},
		{tbl3.Map(func(a Exp) Exp {
			return a.Attr("x")
		}).Reduce(0, func(a, b Exp) Exp {
			return a.Add(b)
		}),
			11,
		},
		{tbl3.Update(func(row Exp) interface{} {
			return Map{"x": Js(`x`)}
		}),
			MatchMap{"errors": 10},
		},
		{tbl3.Update(func(row Exp) interface{} {
			return Map{"x": Js(`x`)}
		}).Atomic(false),
			MatchMap{"errors": 10},
		},
		{tbl3.Map(func(a Exp) Exp {
			return a.Attr("x")
		}).Reduce(0, func(a, b Exp) Exp {
			return a.Add(b)
		}),
			11,
		},
		{tbl3.GetById(0).Update(func(row Exp) interface{} {
			return Map{"x": Js(`x`)}
		}),
			ErrorResponse{},
		},
		{tbl3.GetById(0).Update(func(row Exp) interface{} {
			return Map{"x": Js(`x`)}
		}).Atomic(false),
			ErrorResponse{},
		},
		{tbl3.Map(func(a Exp) Exp {
			return a.Attr("x")
		}).Reduce(0, func(a, b Exp) Exp {
			return a.Add(b)
		}),
			11,
		},
		{tbl3.Update(func(row Exp) interface{} {
			return Branch(Js(`true`), nil, Map{"x": 0.1})
		}),
			MatchMap{"errors": 10},
		},
		{tbl3.Update(func(row Exp) interface{} {
			return Branch(Js(`true`), nil, Map{"x": 0.1})
		}).Atomic(false),
			MatchMap{"skipped": 10},
		},
		{tbl3.Map(func(a Exp) Exp {
			return a.Attr("x")
		}).Reduce(0, func(a, b Exp) Exp {
			return a.Add(b)
		}),
			11,
		},
		{tbl3.GetById(0).Replace(func(row Exp) interface{} {
			return Branch(Js(`true`), row, nil)
		}),
			ErrorResponse{},
		},
		{tbl3.GetById(0).Replace(func(row Exp) interface{} {
			return Branch(Js(`true`), row, nil)
		}).Atomic(false),
			MatchMap{"modified": 1},
		},
		{tbl3.Map(func(a Exp) Exp {
			return a.Attr("x")
		}).Reduce(0, func(a, b Exp) Exp {
			return a.Add(b)
		}),
			11,
		},
		{tbl3.Replace(func(rowA Exp) Exp {
			return Branch(
				Js(fmt.Sprintf("%v.id == 1", rowA)),
				LetVar(rowA.String()).Merge(Map{"x": 2}),
				LetVar(rowA.String()),
			)
		}),
			MatchMap{"errors": 10},
		},
		{tbl3.Replace(func(rowA Exp) Exp {
			return Branch(
				Js(fmt.Sprintf("%v.id == 1", rowA)),
				LetVar(rowA.String()).Merge(Map{"x": 2}),
				LetVar(rowA.String()),
			)
		}).Atomic(false),
			MatchMap{"modified": 10},
		},
		{tbl3.Map(func(a Exp) Exp {
			return a.Attr("x")
		}).Reduce(0, func(a, b Exp) Exp {
			return a.Add(b)
		}),
			12,
		},
		{tbl3.GetById(0).Replace(func(row Exp) interface{} {
			return Branch(Js(`x`), row, nil)
		}),
			ErrorResponse{},
		},
		{tbl3.GetById(0).Replace(func(row Exp) interface{} {
			return Branch(Js(`x`), row, nil)
		}).Atomic(false),
			ErrorResponse{},
		},
		{tbl3.Map(func(a Exp) Exp {
			return a.Attr("x")
		}).Reduce(0, func(a, b Exp) Exp {
			return a.Add(b)
		}),
			12,
		},
		{tbl3.GetById(0).Replace(func(row Exp) interface{} {
			return Branch(Js(`true`), nil, row)
		}),
			ErrorResponse{},
		},
		{tbl3.GetById(0).Replace(func(row Exp) interface{} {
			return Branch(Js(`true`), nil, row)
		}).Atomic(false),
			MatchMap{"deleted": 1},
		},
		{tbl3.Map(func(a Exp) Exp {
			return a.Attr("x")
		}).Reduce(0, func(a, b Exp) Exp {
			return a.Add(b)
		}),
			10,
		},
		{tbl3.Replace(func(rowA Exp) Exp {
			return Branch(
				Js(fmt.Sprintf("%v.id < 3", rowA)),
				nil,
				LetVar(rowA.String()),
			)
		}),
			MatchMap{"errors": 9},
		},
		{tbl3.Replace(func(rowA Exp) Exp {
			return Branch(
				Js(fmt.Sprintf("%v.id < 3", rowA)),
				nil,
				LetVar(rowA.String()),
			)
		}).Atomic(false),
			MatchMap{"deleted": 2},
		},
		{tbl3.Map(func(a Exp) Exp {
			return a.Attr("x")
		}).Reduce(0, func(a, b Exp) Exp {
			return a.Add(b)
		}),
			7,
		},
		{tbl3.GetById(0).Replace(
			Map{
				"id":    0,
				"count": tbl3.GetById(3).Attr("count"),
				"x":     tbl3.GetById(3).Attr("x"),
			}),
			ErrorResponse{},
		},
		{tbl3.GetById(0).Replace(
			Map{
				"id":    0,
				"count": tbl3.GetById(3).Attr("count"),
				"x":     tbl3.GetById(3).Attr("x"),
			}).Atomic(false),
			MatchMap{"inserted": 1},
		},
		{tbl3.GetById(1).Replace(
			tbl3.GetById(3).Merge(Map{"id": 1}),
		),
			ErrorResponse{},
		},
		{tbl3.GetById(1).Replace(
			tbl3.GetById(3).Merge(Map{"id": 1}),
		).Atomic(false),
			MatchMap{"inserted": 1},
		},
		{tbl3.GetById(2).Replace(
			tbl3.GetById(1).Merge(Map{"id": 2}),
		).Atomic(false),
			MatchMap{"inserted": 1},
		},
		{tbl3.Map(func(a Exp) Exp {
			return a.Attr("x")
		}).Reduce(0, func(a, b Exp) Exp {
			return a.Add(b)
		}),
			10,
		},
	},
	"delete": {
		{tbl.GetById(0).Delete(),
			MatchMap{"deleted": 1},
		},
		{tbl.Count(),
			9,
		},
		{tbl.Delete(),
			MatchMap{"deleted": 9},
		},
		{tbl.Count(),
			0,
		},
	},
	"foreach": {
		{Expr(1, 2, 3).ForEach(func(a Exp) Query {
			return tbl4.Insert(Map{"id": a, "fe": true})
		}),
			MatchMap{"inserted": 3},
		},
		{tbl4.ForEach(func(a Exp) Query {
			return tbl4.GetById(a.Attr("id")).Update(Map{"fe": true})
		}),
			MatchMap{"updated": 3},
		},
		{tbl4.Filter(Map{"fe": true}).Count(),
			3,
		},
	},
}

var testStreamGroups = map[string][]ExpectPair{
	"join": {
		{j1.InnerJoin(j2, func(one, two Exp) Exp {
			return one.Attr("id").Eq(two.Attr("id"))
		}).Zip().OrderBy("id"),
			List{
				Map{"id": 0, "name": "bob", "title": "goof"},
				Map{"id": 2, "name": "joe", "title": "lmoe"},
			},
		},
		{j1.OuterJoin(j2, func(one, two Exp) Exp {
			return one.Attr("id").Eq(two.Attr("id"))
		}).Zip().OrderBy("id"),
			List{
				Map{"id": 0, "name": "bob", "title": "goof"},
				Map{"id": 1, "name": "tom"},
				Map{"id": 2, "name": "joe", "title": "lmoe"},
			},
		},
		{j1.EqJoin("id", j2, "id").Zip().OrderBy("id"),
			List{
				Map{"id": 0, "name": "bob", "title": "goof"},
				Map{"id": 2, "name": "joe", "title": "lmoe"},
			},
		},
		{j1.EqJoin("id", j3, "it").Zip().OrderBy("id"),
			List{
				Map{"id": 0, "it": 0, "name": "bob", "title": "goof"},
				Map{"id": 2, "it": 2, "name": "joe", "title": "lmoe"},
			},
		},
	},
}

func (s *RethinkSuite) TestGroups(c *C) {
	for group, pairs := range testSimpleGroups {
		for index, pair := range pairs {
			fmt.Println("group:", group, index)
			runSimpleQuery(c, pair)
		}
		resetDatabase(c)
	}

	for group, pairs := range testStreamGroups {
		for index, pair := range pairs {
			fmt.Println("group:", group, index)
			runStreamQuery(c, pair)
		}
		resetDatabase(c)
	}
}

func (s *RethinkSuite) TestGet(c *C) {
	for i := 0; i < 10; i++ {
		pair := ExpectPair{tbl.GetById(i), Map{"id": i, "num": 20 - i}}
		runSimpleQuery(c, pair)
	}
}

func (s *RethinkSuite) TestOrderBy(c *C) {
	var results1 []Map
	var results2 []Map

	tbl.OrderBy("num").Run(session).Collect(&results1)
	tbl.OrderBy(Asc("num")).Run(session).Collect(&results2)

	c.Assert(results1, JsonEquals, results2)
}

func (s *RethinkSuite) TestDropTable(c *C) {
	err := Db("test").TableCreate("tablex").Run(session).Err()
	c.Assert(err, IsNil)
	err = Db("test").TableDrop("tablex").Run(session).Err()
	c.Assert(err, IsNil)
}
