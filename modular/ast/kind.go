package ast

// Statement node kinds (direct children of source):
//
//   - root_assignment   root[.path]* = <expr>
//   - let_assignment    let <name> = <expr>
//   - meta_assignment   meta [key] = <expr>
//   - map_declaration   map <name> { <statement>* }
//   - import_statement  import "<path>"
//
// Expression node kinds (appear recursively as field "value" or sub-expressions):
//
//   - if_expr            if <cond> { <expr> } [else { <expr> } | else <if_expr>]
//   - match_expr         match [<expr>] { <match_case>* }
//   - match_case           <pattern|catch_all> => <expr>
//   - binary_expr        <expr> (> < >= <= == != && || + - * / % |) <expr>
//   - unary_expr         (! | -) <expr>
//   - method_call        <expr>.<method>(<args...>)
//   - field_access       <expr>.<field>
//   - call_expr          <fn>(<args...>)
//   - lambda             <param> -> <expr>
//   - object_literal     { <pair>* }   where pair holds fields "key" and "value"
//   - array_literal      [ <expr>* ]
//   - parenthesized_expr ( <expr> )
//
// Atom / primary node kinds:
//
//   - this_ref      "this"
//   - meta_ref      @<identifier>
//   - bare_meta_ref "@" with no identifier
//   - variable_ref  $<identifier>
//   - deleted       deleted()
//   - identifier    bare name (field, function, map name, …)
//   - string        "…" or """…"""
//   - number        integer or float
//   - boolean       true | false
//   - catch_all     _ (only inside match_case)
//   - comment       # … (attached as an extra, can appear anywhere)
//
// Named fields on key nodes:
//
//	root_assignment : path*, value
//	let_assignment  : name, value
//	meta_assignment : key (optional), value
//	map_declaration : name
//	if_expr         : condition, consequence, alternative (optional)
//	match_expr      : condition (optional)
//	match_case      : pattern, result
//	binary_expr     : left, operator, right
//	unary_expr      : operator, argument
//	method_call     : object, method
//	field_access    : object, field
//	call_expr       : function
//	lambda          : param, body
//	pair            : key, value

type NodeKind string

const (
	RootAssignment  NodeKind = "root_assignment"
	LetAssignment   NodeKind = "let_assignment"
	MetaAssignment  NodeKind = "meta_assignment"
	MapDeclaration  NodeKind = "map_declaration"
	ImportStatement NodeKind = "import_statement"

	IfExpr        NodeKind = "if_expr"
	MatchExpr     NodeKind = "match_expr"
	MatchCase     NodeKind = "match_case"
	CatchAll      NodeKind = "catch_all"
	BinaryExpr    NodeKind = "binary_expr"
	UnaryExpr     NodeKind = "unary_expr"
	MethodCall    NodeKind = "method_call"
	FieldAccess   NodeKind = "field_access"
	CallExpr      NodeKind = "call_expr"
	Lambda        NodeKind = "lambda"
	ObjectLiteral NodeKind = "object_literal"
	ArrayLiteral  NodeKind = "array_literal"
	Parenthesized NodeKind = "parenthesized_expr"

	ThisRef     NodeKind = "this_ref"
	MetaRef     NodeKind = "meta_ref"
	BareMetaRef NodeKind = "bare_meta_ref"
	VariableRef NodeKind = "variable_ref"
	Deleted     NodeKind = "deleted"
	Identifier  NodeKind = "identifier"
	String      NodeKind = "string"
	Number      NodeKind = "number"
	Boolean     NodeKind = "boolean"
	Comment     NodeKind = "comment"
)
