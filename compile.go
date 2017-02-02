package lua

import (
	"fmt"
	"github.com/yuin/gopher-lua/ast"
	"math"
	"reflect"
)

/* internal constants & structs  {{{ */

const maxRegisters = 200

type expContextType int

const (
	ecGlobal expContextType = iota
	ecUpvalue
	ecLocal
	ecTable
	ecVararg
	ecMethod
	ecNone
)

const regNotDefined = opMaxArgsA + 1
const labelNoJump = 0

type expcontext struct {
	ctype expContextType
	reg   int
	// varargopt >= 0: wants varargopt+1 results, i.e  a = func()
	// varargopt = -1: ignore results             i.e  func()
	// varargopt = -2: receive all results        i.e  a = {func()}
	varargopt int
}

type assigncontext struct {
	ec       *expcontext
	keyrk    int
	valuerk  int
	keyks    bool
	needmove bool
}

type lblabels struct {
	t int
	f int
	e int
	b bool
}

type constLValueExpr struct {
	ast.ExprBase

	Value LValue
}

// }}}

/* utilities {{{ */
var _ecnone0 = &expcontext{ecNone, regNotDefined, 0}
var _ecnonem1 = &expcontext{ecNone, regNotDefined, -1}
var _ecnonem2 = &expcontext{ecNone, regNotDefined, -2}
var ecfuncdef = &expcontext{ecMethod, regNotDefined, 0}

func ecupdate(ec *expcontext, ctype expContextType, reg, varargopt int) {
	ec.ctype = ctype
	ec.reg = reg
	ec.varargopt = varargopt
}

func ecnone(varargopt int) *expcontext {
	switch varargopt {
	case 0:
		return _ecnone0
	case -1:
		return _ecnonem1
	case -2:
		return _ecnonem2
	}
	return &expcontext{ecNone, regNotDefined, varargopt}
}

func sline(pos ast.PositionHolder) int {
	return pos.Line()
}

func eline(pos ast.PositionHolder) int {
	return pos.LastLine()
}

func savereg(ec *expcontext, reg int) int {
	if ec.ctype != ecLocal || ec.reg == regNotDefined {
		return reg
	}
	return ec.reg
}

func raiseCompileError(context *funcContext, line int, format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	panic(&CompileError{context: context, Line: line, Message: msg})
}

func isVarArgReturnExpr(expr ast.Expr) bool {
	switch ex := expr.(type) {
	case *ast.FuncCallExpr:
		return !ex.AdjustRet
	case *ast.Comma3Expr:
		return true
	}
	return false
}

func lnumberValue(expr ast.Expr) (LNumber, bool) {
	if ex, ok := expr.(*ast.NumberExpr); ok {
		lv, err := parseNumber(ex.Value)
		if err != nil {
			lv = LNumber(math.NaN())
		}
		return lv, true
	} else if ex, ok := expr.(*constLValueExpr); ok {
		return ex.Value.(LNumber), true
	}
	return 0, false
}

/* utilities }}} */

type CompileError struct { // {{{
	context *funcContext
	Line    int
	Message string
}

func (e *CompileError) Error() string {
	return fmt.Sprintf("compile error near line(%v) %v: %v", e.Line, e.context.proto.SourceName, e.Message)
} // }}}

type codeStore struct { // {{{
	codes []uint32
	lines []int
	pc    int
}

func (cd *codeStore) Add(inst uint32, line int) {
	if l := len(cd.codes); l <= 0 || cd.pc == l {
		cd.codes = append(cd.codes, inst)
		cd.lines = append(cd.lines, line)
	} else {
		cd.codes[cd.pc] = inst
		cd.lines[cd.pc] = line
	}
	cd.pc++
}

func (cd *codeStore) AddABC(op int, a int, b int, c int, line int) {
	cd.Add(opCreateABC(op, a, b, c), line)
}

func (cd *codeStore) AddABx(op int, a int, bx int, line int) {
	cd.Add(opCreateABx(op, a, bx), line)
}

func (cd *codeStore) AddASbx(op int, a int, sbx int, line int) {
	cd.Add(opCreateASbx(op, a, sbx), line)
}

func (cd *codeStore) PropagateKMV(top int, save *int, reg *int, inc int) {
	lastinst := cd.Last()
	if opGetArgA(lastinst) >= top {
		switch opGetOpCode(lastinst) {
		case op_LOADK:
			cindex := opGetArgBx(lastinst)
			if cindex <= opMaxIndexRk {
				cd.Pop()
				*save = opRkAsk(cindex)
				return
			}
		case op_MOVE:
			cd.Pop()
			*save = opGetArgB(lastinst)
			return
		}
	}
	*save = *reg
	*reg = *reg + inc
}

func (cd *codeStore) PropagateMV(top int, save *int, reg *int, inc int) {
	lastinst := cd.Last()
	if opGetArgA(lastinst) >= top {
		switch opGetOpCode(lastinst) {
		case op_MOVE:
			cd.Pop()
			*save = opGetArgB(lastinst)
			return
		}
	}
	*save = *reg
	*reg = *reg + inc
}

func (cd *codeStore) SetOpCode(pc int, v int) {
	opSetOpCode(&cd.codes[pc], v)
}

func (cd *codeStore) SetA(pc int, v int) {
	opSetArgA(&cd.codes[pc], v)
}

func (cd *codeStore) SetB(pc int, v int) {
	opSetArgB(&cd.codes[pc], v)
}

func (cd *codeStore) SetC(pc int, v int) {
	opSetArgC(&cd.codes[pc], v)
}

func (cd *codeStore) SetBx(pc int, v int) {
	opSetArgBx(&cd.codes[pc], v)
}

func (cd *codeStore) SetSbx(pc int, v int) {
	opSetArgSbx(&cd.codes[pc], v)
}

func (cd *codeStore) At(pc int) uint32 {
	return cd.codes[pc]
}

func (cd *codeStore) List() []uint32 {
	return cd.codes[:cd.pc]
}

func (cd *codeStore) PosList() []int {
	return cd.lines[:cd.pc]
}

func (cd *codeStore) LastPC() int {
	return cd.pc - 1
}

func (cd *codeStore) Last() uint32 {
	if cd.pc == 0 {
		return opInvalidInstruction
	}
	return cd.codes[cd.pc-1]
}

func (cd *codeStore) Pop() {
	cd.pc--
} /* }}} Code */

/* {{{ VarNamePool */

type varNamePoolValue struct {
	index int
	name  string
}

type varNamePool struct {
	names  []string
	offset int
}

func newVarNamePool(offset int) *varNamePool {
	return &varNamePool{make([]string, 0, 16), offset}
}

func (vp *varNamePool) Names() []string {
	return vp.names
}

func (vp *varNamePool) List() []varNamePoolValue {
	result := make([]varNamePoolValue, len(vp.names), len(vp.names))
	for i, name := range vp.names {
		result[i].index = i + vp.offset
		result[i].name = name
	}
	return result
}

func (vp *varNamePool) LastIndex() int {
	return vp.offset + len(vp.names)
}

func (vp *varNamePool) Find(name string) int {
	for i := len(vp.names) - 1; i >= 0; i-- {
		if vp.names[i] == name {
			return i + vp.offset
		}
	}
	return -1
}

func (vp *varNamePool) RegisterUnique(name string) int {
	index := vp.Find(name)
	if index < 0 {
		return vp.Register(name)
	}
	return index
}

func (vp *varNamePool) Register(name string) int {
	vp.names = append(vp.names, name)
	return len(vp.names) - 1 + vp.offset
}

/* }}} VarNamePool */

/* FuncContext {{{ */

type codeBlock struct {
	localVars  *varNamePool
	breakLabel int
	parent     *codeBlock
	refUpvalue bool
	lineStart  int
	lastLine   int
}

func newCodeBlock(localvars *varNamePool, blabel int, parent *codeBlock, pos ast.PositionHolder) *codeBlock {
	bl := &codeBlock{localvars, blabel, parent, false, 0, 0}
	if pos != nil {
		bl.lineStart = pos.Line()
		bl.lastLine = pos.LastLine()
	}
	return bl
}

type funcContext struct {
	proto    *FunctionProto
	code     *codeStore
	parent   *funcContext
	upvalues *varNamePool
	block    *codeBlock
	blocks   []*codeBlock
	regTop   int
	labelId  int
	labelPc  map[int]int
}

func newFuncContext(sourcename string, parent *funcContext) *funcContext {
	fc := &funcContext{
		proto:    newFunctionProto(sourcename),
		code:     &codeStore{make([]uint32, 0, 1024), make([]int, 0, 1024), 0},
		parent:   parent,
		upvalues: newVarNamePool(0),
		block:    newCodeBlock(newVarNamePool(0), labelNoJump, nil, nil),
		regTop:   0,
		labelId:  1,
		labelPc:  map[int]int{},
	}
	fc.blocks = []*codeBlock{fc.block}
	return fc
}

func (fc *funcContext) NewLabel() int {
	ret := fc.labelId
	fc.labelId++
	return ret
}

func (fc *funcContext) SetLabelPc(label int, pc int) {
	fc.labelPc[label] = pc
}

func (fc *funcContext) GetLabelPc(label int) int {
	return fc.labelPc[label]
}

func (fc *funcContext) ConstIndex(value LValue) int {
	ctype := value.Type()
	for i, lv := range fc.proto.Constants {
		if lv.Type() == ctype && lv == value {
			return i
		}
	}
	fc.proto.Constants = append(fc.proto.Constants, value)
	v := len(fc.proto.Constants) - 1
	if v > opMaxArgBx {
		raiseCompileError(fc, fc.proto.LineDefined, "too many constants")
	}
	return v
}

func (fc *funcContext) RegisterLocalVar(name string) int {
	ret := fc.block.localVars.Register(name)
	fc.proto.DbgLocals = append(fc.proto.DbgLocals, &DbgLocalInfo{Name: name, StartPc: fc.code.LastPC() + 1})
	fc.SetRegTop(fc.RegTop() + 1)
	return ret
}

func (fc *funcContext) FindLocalVarAndBlock(name string) (int, *codeBlock) {
	for block := fc.block; block != nil; block = block.parent {
		if index := block.localVars.Find(name); index > -1 {
			return index, block
		}
	}
	return -1, nil
}

func (fc *funcContext) FindLocalVar(name string) int {
	idx, _ := fc.FindLocalVarAndBlock(name)
	return idx
}

func (fc *funcContext) LocalVars() []varNamePoolValue {
	result := make([]varNamePoolValue, 0, 32)
	for _, block := range fc.blocks {
		result = append(result, block.localVars.List()...)
	}
	return result
}

func (fc *funcContext) EnterBlock(blabel int, pos ast.PositionHolder) {
	fc.block = newCodeBlock(newVarNamePool(fc.RegTop()), blabel, fc.block, pos)
	fc.blocks = append(fc.blocks, fc.block)
}

func (fc *funcContext) CloseUpvalues() int {
	n := -1
	if fc.block.refUpvalue {
		n = fc.block.parent.localVars.LastIndex()
		fc.code.AddABC(op_CLOSE, n, 0, 0, fc.block.lastLine)
	}
	return n
}

func (fc *funcContext) LeaveBlock() int {
	closed := fc.CloseUpvalues()
	fc.EndScope()
	fc.block = fc.block.parent
	fc.SetRegTop(fc.block.localVars.LastIndex())
	return closed
}

func (fc *funcContext) EndScope() {
	for _, vr := range fc.block.localVars.List() {
		fc.proto.DbgLocals[vr.index].EndPc = fc.code.LastPC()
	}
}

func (fc *funcContext) SetRegTop(top int) {
	if top > maxRegisters {
		raiseCompileError(fc, fc.proto.LineDefined, "too many local variables")
	}
	fc.regTop = top
}

func (fc *funcContext) RegTop() int {
	return fc.regTop
}

/* FuncContext }}} */

func compileChunk(context *funcContext, chunk []ast.Stmt) { // {{{
	for _, stmt := range chunk {
		compileStmt(context, stmt)
	}
} // }}}

func compileBlock(context *funcContext, chunk []ast.Stmt) { // {{{
	if len(chunk) == 0 {
		return
	}
	ph := &ast.Node{}
	ph.SetLine(sline(chunk[0]))
	ph.SetLastLine(eline(chunk[len(chunk)-1]))
	context.EnterBlock(labelNoJump, ph)
	for _, stmt := range chunk {
		compileStmt(context, stmt)
	}
	context.LeaveBlock()
} // }}}

func compileStmt(context *funcContext, stmt ast.Stmt) { // {{{
	switch st := stmt.(type) {
	case *ast.AssignStmt:
		compileAssignStmt(context, st)
	case *ast.LocalAssignStmt:
		compileLocalAssignStmt(context, st)
	case *ast.FuncCallStmt:
		compileFuncCallExpr(context, context.RegTop(), st.Expr.(*ast.FuncCallExpr), ecnone(-1))
	case *ast.DoBlockStmt:
		context.EnterBlock(labelNoJump, st)
		compileChunk(context, st.Stmts)
		context.LeaveBlock()
	case *ast.WhileStmt:
		compileWhileStmt(context, st)
	case *ast.RepeatStmt:
		compileRepeatStmt(context, st)
	case *ast.FuncDefStmt:
		compileFuncDefStmt(context, st)
	case *ast.ReturnStmt:
		compileReturnStmt(context, st)
	case *ast.IfStmt:
		compileIfStmt(context, st)
	case *ast.BreakStmt:
		compileBreakStmt(context, st)
	case *ast.NumberForStmt:
		compileNumberForStmt(context, st)
	case *ast.GenericForStmt:
		compileGenericForStmt(context, st)
	}
} // }}}

func compileAssignStmtLeft(context *funcContext, stmt *ast.AssignStmt) (int, []*assigncontext) { // {{{
	reg := context.RegTop()
	acs := make([]*assigncontext, 0, len(stmt.Lhs))
	for i, lhs := range stmt.Lhs {
		islast := i == len(stmt.Lhs)-1
		switch st := lhs.(type) {
		case *ast.IdentExpr:
			identtype := getIdentRefType(context, context, st)
			ec := &expcontext{identtype, regNotDefined, 0}
			switch identtype {
			case ecGlobal:
				context.ConstIndex(LString(st.Value))
			case ecUpvalue:
				context.upvalues.RegisterUnique(st.Value)
			case ecLocal:
				if islast {
					ec.reg = context.FindLocalVar(st.Value)
				}
			}
			acs = append(acs, &assigncontext{ec, 0, 0, false, false})
		case *ast.AttrGetExpr:
			ac := &assigncontext{&expcontext{ecTable, regNotDefined, 0}, 0, 0, false, false}
			compileExprWithKMVPropagation(context, st.Object, &reg, &ac.ec.reg)
			compileExprWithKMVPropagation(context, st.Key, &reg, &ac.keyrk)
			if _, ok := st.Key.(*ast.StringExpr); ok {
				ac.keyks = true
			}
			acs = append(acs, ac)

		default:
			panic("invalid left expression.")
		}
	}
	return reg, acs
} // }}}

func compileAssignStmtRight(context *funcContext, stmt *ast.AssignStmt, reg int, acs []*assigncontext) (int, []*assigncontext) { // {{{
	lennames := len(stmt.Lhs)
	lenexprs := len(stmt.Rhs)
	namesassigned := 0

	for namesassigned < lennames {
		ac := acs[namesassigned]
		ec := ac.ec
		var expr ast.Expr = nil
		if namesassigned >= lenexprs {
			expr = &ast.NilExpr{}
			expr.SetLine(sline(stmt.Lhs[namesassigned]))
			expr.SetLastLine(eline(stmt.Lhs[namesassigned]))
		} else if isVarArgReturnExpr(stmt.Rhs[namesassigned]) && (lenexprs-namesassigned-1) <= 0 {
			varargopt := lennames - namesassigned - 1
			regstart := reg
			reginc := compileExpr(context, reg, stmt.Rhs[namesassigned], ecnone(varargopt))
			reg += reginc
			for i := namesassigned; i < namesassigned+int(reginc); i++ {
				acs[i].needmove = true
				if acs[i].ec.ctype == ecTable {
					acs[i].valuerk = regstart + (i - namesassigned)
				}
			}
			namesassigned = lennames
			continue
		}

		if expr == nil {
			expr = stmt.Rhs[namesassigned]
		}
		idx := reg
		reginc := compileExpr(context, reg, expr, ec)
		if ec.ctype == ecTable {
			if _, ok := expr.(*ast.LogicalOpExpr); !ok {
				context.code.PropagateKMV(context.RegTop(), &ac.valuerk, &reg, reginc)
			} else {
				ac.valuerk = idx
				reg += reginc
			}
		} else {
			ac.needmove = reginc != 0
			reg += reginc
		}
		namesassigned += 1
	}

	rightreg := reg - 1

	// extra right exprs
	for i := namesassigned; i < lenexprs; i++ {
		varargopt := -1
		if i != lenexprs-1 {
			varargopt = 0
		}
		reg += compileExpr(context, reg, stmt.Rhs[i], ecnone(varargopt))
	}
	return rightreg, acs
} // }}}

func compileAssignStmt(context *funcContext, stmt *ast.AssignStmt) { // {{{
	code := context.code
	lennames := len(stmt.Lhs)
	reg, acs := compileAssignStmtLeft(context, stmt)
	reg, acs = compileAssignStmtRight(context, stmt, reg, acs)

	for i := lennames - 1; i >= 0; i-- {
		ex := stmt.Lhs[i]
		switch acs[i].ec.ctype {
		case ecLocal:
			if acs[i].needmove {
				code.AddABC(op_MOVE, context.FindLocalVar(ex.(*ast.IdentExpr).Value), reg, 0, sline(ex))
				reg -= 1
			}
		case ecGlobal:
			code.AddABx(op_SETGLOBAL, reg, context.ConstIndex(LString(ex.(*ast.IdentExpr).Value)), sline(ex))
			reg -= 1
		case ecUpvalue:
			code.AddABC(op_SETUPVAL, reg, context.upvalues.RegisterUnique(ex.(*ast.IdentExpr).Value), 0, sline(ex))
			reg -= 1
		case ecTable:
			opcode := op_SETTABLE
			if acs[i].keyks {
				opcode = op_SETTABLEKS
			}
			code.AddABC(opcode, acs[i].ec.reg, acs[i].keyrk, acs[i].valuerk, sline(ex))
			if !opIsK(acs[i].valuerk) {
				reg -= 1
			}
		}
	}
} // }}}

func compileRegAssignment(context *funcContext, names []string, exprs []ast.Expr, reg int, nvars int, line int) { // {{{
	lennames := len(names)
	lenexprs := len(exprs)
	namesassigned := 0
	ec := &expcontext{}

	for namesassigned < lennames && namesassigned < lenexprs {
		if isVarArgReturnExpr(exprs[namesassigned]) && (lenexprs-namesassigned-1) <= 0 {

			varargopt := nvars - namesassigned
			ecupdate(ec, ecVararg, reg, varargopt-1)
			compileExpr(context, reg, exprs[namesassigned], ec)
			reg += varargopt
			namesassigned = lennames
		} else {
			ecupdate(ec, ecLocal, reg, 0)
			compileExpr(context, reg, exprs[namesassigned], ec)
			reg += 1
			namesassigned += 1
		}
	}

	// extra left names
	if lennames > namesassigned {
		restleft := lennames - namesassigned - 1
		context.code.AddABC(op_LOADNIL, reg, reg+restleft, 0, line)
		reg += restleft
	}

	// extra right exprs
	for i := namesassigned; i < lenexprs; i++ {
		varargopt := -1
		if i != lenexprs-1 {
			varargopt = 0
		}
		ecupdate(ec, ecNone, reg, varargopt)
		reg += compileExpr(context, reg, exprs[i], ec)
	}
} // }}}

func compileLocalAssignStmt(context *funcContext, stmt *ast.LocalAssignStmt) { // {{{
	reg := context.RegTop()
	if len(stmt.Names) == 1 && len(stmt.Exprs) == 1 {
		if _, ok := stmt.Exprs[0].(*ast.FunctionExpr); ok {
			context.RegisterLocalVar(stmt.Names[0])
			compileRegAssignment(context, stmt.Names, stmt.Exprs, reg, len(stmt.Names), sline(stmt))
			return
		}
	}

	compileRegAssignment(context, stmt.Names, stmt.Exprs, reg, len(stmt.Names), sline(stmt))
	for _, name := range stmt.Names {
		context.RegisterLocalVar(name)
	}
} // }}}

func compileReturnStmt(context *funcContext, stmt *ast.ReturnStmt) { // {{{
	lenexprs := len(stmt.Exprs)
	code := context.code
	reg := context.RegTop()
	a := reg
	lastisvaarg := false

	if lenexprs == 1 {
		switch ex := stmt.Exprs[0].(type) {
		case *ast.IdentExpr:
			if idx := context.FindLocalVar(ex.Value); idx > -1 {
				code.AddABC(op_RETURN, idx, 2, 0, sline(stmt))
				return
			}
		case *ast.FuncCallExpr:
			reg += compileExpr(context, reg, ex, ecnone(-2))
			code.SetOpCode(code.LastPC(), op_TAILCALL)
			code.AddABC(op_RETURN, a, 0, 0, sline(stmt))
			return
		}
	}

	for i, expr := range stmt.Exprs {
		if i == lenexprs-1 && isVarArgReturnExpr(expr) {
			compileExpr(context, reg, expr, ecnone(-2))
			lastisvaarg = true
		} else {
			reg += compileExpr(context, reg, expr, ecnone(0))
		}
	}
	count := reg - a + 1
	if lastisvaarg {
		count = 0
	}
	context.code.AddABC(op_RETURN, a, count, 0, sline(stmt))
} // }}}

func compileIfStmt(context *funcContext, stmt *ast.IfStmt) { // {{{
	thenlabel := context.NewLabel()
	elselabel := context.NewLabel()
	endlabel := context.NewLabel()

	compileBranchCondition(context, context.RegTop(), stmt.Condition, thenlabel, elselabel, false)
	context.SetLabelPc(thenlabel, context.code.LastPC())
	compileBlock(context, stmt.Then)
	if len(stmt.Else) > 0 {
		context.code.AddASbx(op_JMP, 0, endlabel, sline(stmt))
	}
	context.SetLabelPc(elselabel, context.code.LastPC())
	if len(stmt.Else) > 0 {
		compileBlock(context, stmt.Else)
		context.SetLabelPc(endlabel, context.code.LastPC())
	}

} // }}}

func compileBranchCondition(context *funcContext, reg int, expr ast.Expr, thenlabel, elselabel int, hasnextcond bool) { // {{{
	// TODO folding constants?
	code := context.code
	flip := 0
	jumplabel := elselabel
	if hasnextcond {
		flip = 1
		jumplabel = thenlabel
	}

	switch ex := expr.(type) {
	case *ast.FalseExpr, *ast.NilExpr:
		if !hasnextcond {
			code.AddASbx(op_JMP, 0, elselabel, sline(expr))
			return
		}
	case *ast.TrueExpr, *ast.NumberExpr, *ast.StringExpr:
		if !hasnextcond {
			return
		}
	case *ast.UnaryNotOpExpr:
		compileBranchCondition(context, reg, ex.Expr, elselabel, thenlabel, !hasnextcond)
		return
	case *ast.LogicalOpExpr:
		switch ex.Operator {
		case "and":
			nextcondlabel := context.NewLabel()
			compileBranchCondition(context, reg, ex.Lhs, nextcondlabel, elselabel, false)
			context.SetLabelPc(nextcondlabel, context.code.LastPC())
			compileBranchCondition(context, reg, ex.Rhs, thenlabel, elselabel, hasnextcond)
		case "or":
			nextcondlabel := context.NewLabel()
			compileBranchCondition(context, reg, ex.Lhs, thenlabel, nextcondlabel, true)
			context.SetLabelPc(nextcondlabel, context.code.LastPC())
			compileBranchCondition(context, reg, ex.Rhs, thenlabel, elselabel, hasnextcond)
		}
		return
	case *ast.RelationalOpExpr:
		compileRelationalOpExprAux(context, reg, ex, flip, jumplabel)
		return
	}

	a := reg
	compileExprWithMVPropagation(context, expr, &reg, &a)
	code.AddABC(op_TEST, a, 0, 0^flip, sline(expr))
	code.AddASbx(op_JMP, 0, jumplabel, sline(expr))
} // }}}

func compileWhileStmt(context *funcContext, stmt *ast.WhileStmt) { // {{{
	thenlabel := context.NewLabel()
	elselabel := context.NewLabel()
	condlabel := context.NewLabel()

	context.SetLabelPc(condlabel, context.code.LastPC())
	compileBranchCondition(context, context.RegTop(), stmt.Condition, thenlabel, elselabel, false)
	context.SetLabelPc(thenlabel, context.code.LastPC())
	context.EnterBlock(elselabel, stmt)
	compileChunk(context, stmt.Stmts)
	context.CloseUpvalues()
	context.code.AddASbx(op_JMP, 0, condlabel, eline(stmt))
	context.LeaveBlock()
	context.SetLabelPc(elselabel, context.code.LastPC())
} // }}}

func compileRepeatStmt(context *funcContext, stmt *ast.RepeatStmt) { // {{{
	initlabel := context.NewLabel()
	thenlabel := context.NewLabel()
	elselabel := context.NewLabel()

	context.SetLabelPc(initlabel, context.code.LastPC())
	context.SetLabelPc(elselabel, context.code.LastPC())
	context.EnterBlock(thenlabel, stmt)
	compileChunk(context, stmt.Stmts)
	compileBranchCondition(context, context.RegTop(), stmt.Condition, thenlabel, elselabel, false)

	context.SetLabelPc(thenlabel, context.code.LastPC())
	n := context.LeaveBlock()

	if n > -1 {
		label := context.NewLabel()
		context.code.AddASbx(op_JMP, 0, label, eline(stmt))
		context.SetLabelPc(elselabel, context.code.LastPC())
		context.code.AddABC(op_CLOSE, n, 0, 0, eline(stmt))
		context.code.AddASbx(op_JMP, 0, initlabel, eline(stmt))
		context.SetLabelPc(label, context.code.LastPC())
	}

} // }}}

func compileBreakStmt(context *funcContext, stmt *ast.BreakStmt) { // {{{
	for block := context.block; block != nil; block = block.parent {
		if label := block.breakLabel; label != labelNoJump {
			if block.refUpvalue {
				context.code.AddABC(op_CLOSE, block.parent.localVars.LastIndex(), 0, 0, sline(stmt))
			}
			context.code.AddASbx(op_JMP, 0, label, sline(stmt))
			return
		}
	}
	raiseCompileError(context, sline(stmt), "no loop to break")
} // }}}

func compileFuncDefStmt(context *funcContext, stmt *ast.FuncDefStmt) { // {{{
	if stmt.Name.Func == nil {
		reg := context.RegTop()
		var treg, kreg int
		compileExprWithKMVPropagation(context, stmt.Name.Receiver, &reg, &treg)
		kreg = loadRk(context, &reg, stmt.Func, LString(stmt.Name.Method))
		compileExpr(context, reg, stmt.Func, ecfuncdef)
		context.code.AddABC(op_SETTABLE, treg, kreg, reg, sline(stmt.Name.Receiver))
	} else {
		astmt := &ast.AssignStmt{Lhs: []ast.Expr{stmt.Name.Func}, Rhs: []ast.Expr{stmt.Func}}
		astmt.SetLine(sline(stmt.Func))
		astmt.SetLastLine(eline(stmt.Func))
		compileAssignStmt(context, astmt)
	}
} // }}}

func compileNumberForStmt(context *funcContext, stmt *ast.NumberForStmt) { // {{{
	code := context.code
	endlabel := context.NewLabel()
	ec := &expcontext{}

	context.EnterBlock(endlabel, stmt)
	reg := context.RegTop()
	rindex := context.RegisterLocalVar("(for index)")
	ecupdate(ec, ecLocal, rindex, 0)
	compileExpr(context, reg, stmt.Init, ec)

	reg = context.RegTop()
	rlimit := context.RegisterLocalVar("(for limit)")
	ecupdate(ec, ecLocal, rlimit, 0)
	compileExpr(context, reg, stmt.Limit, ec)

	reg = context.RegTop()
	rstep := context.RegisterLocalVar("(for step)")
	if stmt.Step == nil {
		stmt.Step = &ast.NumberExpr{Value: "1"}
		stmt.Step.SetLine(sline(stmt.Init))
	}
	ecupdate(ec, ecLocal, rstep, 0)
	compileExpr(context, reg, stmt.Step, ec)

	code.AddASbx(op_FORPREP, rindex, 0, sline(stmt))

	context.RegisterLocalVar(stmt.Name)

	bodypc := code.LastPC()
	compileChunk(context, stmt.Stmts)

	context.LeaveBlock()

	flpc := code.LastPC()
	code.AddASbx(op_FORLOOP, rindex, bodypc-(flpc+1), sline(stmt))

	context.SetLabelPc(endlabel, code.LastPC())
	code.SetSbx(bodypc, flpc-bodypc)

} // }}}

func compileGenericForStmt(context *funcContext, stmt *ast.GenericForStmt) { // {{{
	code := context.code
	endlabel := context.NewLabel()
	bodylabel := context.NewLabel()
	fllabel := context.NewLabel()
	nnames := len(stmt.Names)

	context.EnterBlock(endlabel, stmt)
	rgen := context.RegisterLocalVar("(for generator)")
	context.RegisterLocalVar("(for state)")
	context.RegisterLocalVar("(for control)")

	compileRegAssignment(context, stmt.Names, stmt.Exprs, context.RegTop()-3, 3, sline(stmt))

	code.AddASbx(op_JMP, 0, fllabel, sline(stmt))

	for _, name := range stmt.Names {
		context.RegisterLocalVar(name)
	}

	context.SetLabelPc(bodylabel, code.LastPC())
	compileChunk(context, stmt.Stmts)

	context.LeaveBlock()

	context.SetLabelPc(fllabel, code.LastPC())
	code.AddABC(op_TFORLOOP, rgen, 0, nnames, sline(stmt))
	code.AddASbx(op_JMP, 0, bodylabel, sline(stmt))

	context.SetLabelPc(endlabel, code.LastPC())
} // }}}

func compileExpr(context *funcContext, reg int, expr ast.Expr, ec *expcontext) int { // {{{
	code := context.code
	sreg := savereg(ec, reg)
	sused := 1
	if sreg < reg {
		sused = 0
	}

	switch ex := expr.(type) {
	case *ast.StringExpr:
		code.AddABx(op_LOADK, sreg, context.ConstIndex(LString(ex.Value)), sline(ex))
		return sused
	case *ast.NumberExpr:
		num, err := parseNumber(ex.Value)
		if err != nil {
			num = LNumber(math.NaN())
		}
		code.AddABx(op_LOADK, sreg, context.ConstIndex(num), sline(ex))
		return sused
	case *constLValueExpr:
		code.AddABx(op_LOADK, sreg, context.ConstIndex(ex.Value), sline(ex))
		return sused
	case *ast.NilExpr:
		code.AddABC(op_LOADNIL, sreg, sreg, 0, sline(ex))
		return sused
	case *ast.FalseExpr:
		code.AddABC(op_LOADBOOL, sreg, 0, 0, sline(ex))
		return sused
	case *ast.TrueExpr:
		code.AddABC(op_LOADBOOL, sreg, 1, 0, sline(ex))
		return sused
	case *ast.IdentExpr:
		switch getIdentRefType(context, context, ex) {
		case ecGlobal:
			code.AddABx(op_GETGLOBAL, sreg, context.ConstIndex(LString(ex.Value)), sline(ex))
		case ecUpvalue:
			code.AddABC(op_GETUPVAL, sreg, context.upvalues.RegisterUnique(ex.Value), 0, sline(ex))
		case ecLocal:
			b := context.FindLocalVar(ex.Value)
			code.AddABC(op_MOVE, sreg, b, 0, sline(ex))
		}
		return sused
	case *ast.Comma3Expr:
		if context.proto.IsVarArg == 0 {
			raiseCompileError(context, sline(ex), "cannot use '...' outside a vararg function")
		}
		context.proto.IsVarArg &= ^VarArgNeedsArg
		code.AddABC(op_VARARG, sreg, 2+ec.varargopt, 0, sline(ex))
		if context.RegTop() > (sreg+2+ec.varargopt) || ec.varargopt < -1 {
			return 0
		}
		return (sreg + 1 + ec.varargopt) - reg
	case *ast.AttrGetExpr:
		a := sreg
		b := reg
		compileExprWithMVPropagation(context, ex.Object, &reg, &b)
		c := reg
		compileExprWithKMVPropagation(context, ex.Key, &reg, &c)
		opcode := op_GETTABLE
		if _, ok := ex.Key.(*ast.StringExpr); ok {
			opcode = op_GETTABLEKS
		}
		code.AddABC(opcode, a, b, c, sline(ex))
		return sused
	case *ast.TableExpr:
		compileTableExpr(context, reg, ex, ec)
		return 1
	case *ast.ArithmeticOpExpr:
		compileArithmeticOpExpr(context, reg, ex, ec)
		return sused
	case *ast.StringConcatOpExpr:
		compileStringConcatOpExpr(context, reg, ex, ec)
		return sused
	case *ast.UnaryMinusOpExpr, *ast.UnaryNotOpExpr, *ast.UnaryLenOpExpr:
		compileUnaryOpExpr(context, reg, ex, ec)
		return sused
	case *ast.RelationalOpExpr:
		compileRelationalOpExpr(context, reg, ex, ec)
		return sused
	case *ast.LogicalOpExpr:
		compileLogicalOpExpr(context, reg, ex, ec)
		return sused
	case *ast.FuncCallExpr:
		return compileFuncCallExpr(context, reg, ex, ec)
	case *ast.FunctionExpr:
		childcontext := newFuncContext(context.proto.SourceName, context)
		compileFunctionExpr(childcontext, ex, ec)
		protono := len(context.proto.FunctionPrototypes)
		context.proto.FunctionPrototypes = append(context.proto.FunctionPrototypes, childcontext.proto)
		code.AddABx(op_CLOSURE, sreg, protono, sline(ex))
		for _, upvalue := range childcontext.upvalues.List() {
			localidx, block := context.FindLocalVarAndBlock(upvalue.name)
			if localidx > -1 {
				code.AddABC(op_MOVE, 0, localidx, 0, sline(ex))
				block.refUpvalue = true
			} else {
				upvalueidx := context.upvalues.Find(upvalue.name)
				if upvalueidx < 0 {
					upvalueidx = context.upvalues.RegisterUnique(upvalue.name)
				}
				code.AddABC(op_GETUPVAL, 0, upvalueidx, 0, sline(ex))
			}
		}
		return sused
	default:
		panic(fmt.Sprintf("expr %v not implemented.", reflect.TypeOf(ex).Elem().Name()))
	}

	panic("should not reach here")
	return sused
} // }}}

func compileExprWithPropagation(context *funcContext, expr ast.Expr, reg *int, save *int, propergator func(int, *int, *int, int)) { // {{{
	reginc := compileExpr(context, *reg, expr, ecnone(0))
	if _, ok := expr.(*ast.LogicalOpExpr); ok {
		*save = *reg
		*reg = *reg + reginc
	} else {
		propergator(context.RegTop(), save, reg, reginc)
	}
} // }}}

func compileExprWithKMVPropagation(context *funcContext, expr ast.Expr, reg *int, save *int) { // {{{
	compileExprWithPropagation(context, expr, reg, save, context.code.PropagateKMV)
} // }}}

func compileExprWithMVPropagation(context *funcContext, expr ast.Expr, reg *int, save *int) { // {{{
	compileExprWithPropagation(context, expr, reg, save, context.code.PropagateMV)
} // }}}

func constFold(exp ast.Expr) ast.Expr { // {{{
	switch expr := exp.(type) {
	case *ast.ArithmeticOpExpr:
		lvalue, lisconst := lnumberValue(expr.Lhs)
		rvalue, risconst := lnumberValue(expr.Rhs)
		if lisconst && risconst {
			switch expr.Operator {
			case "+":
				return &constLValueExpr{Value: lvalue + rvalue}
			case "-":
				return &constLValueExpr{Value: lvalue - rvalue}
			case "*":
				return &constLValueExpr{Value: lvalue * rvalue}
			case "/":
				return &constLValueExpr{Value: lvalue / rvalue}
			case "%":
				return &constLValueExpr{Value: luaModulo(lvalue, rvalue)}
			case "^":
				return &constLValueExpr{Value: LNumber(math.Pow(float64(lvalue), float64(rvalue)))}
			default:
				panic(fmt.Sprintf("unknwon binop: %v", expr.Operator))
			}
		} else {
			retexpr := *expr
			retexpr.Lhs = constFold(expr.Lhs)
			retexpr.Rhs = constFold(expr.Rhs)
			return &retexpr
		}
	case *ast.UnaryMinusOpExpr:
		expr.Expr = constFold(expr.Expr)
		if value, ok := lnumberValue(expr.Expr); ok {
			return &constLValueExpr{Value: LNumber(-value)}
		}
		return expr
	default:

		return exp
	}
	return exp
} // }}}

func compileFunctionExpr(context *funcContext, funcexpr *ast.FunctionExpr, ec *expcontext) { // {{{
	context.proto.LineDefined = sline(funcexpr)
	context.proto.LastLineDefined = eline(funcexpr)
	if len(funcexpr.ParList.Names) > maxRegisters {
		raiseCompileError(context, context.proto.LineDefined, "register overflow")
	}
	context.proto.NumParameters = uint8(len(funcexpr.ParList.Names))
	if ec.ctype == ecMethod {
		context.proto.NumParameters += 1
		context.RegisterLocalVar("self")
	}
	for _, name := range funcexpr.ParList.Names {
		context.RegisterLocalVar(name)
	}
	if funcexpr.ParList.HasVargs {
		if CompatVarArg {
			context.proto.IsVarArg = VarArgHasArg | VarArgNeedsArg
			if context.parent != nil {
				context.RegisterLocalVar("arg")
			}
		}
		context.proto.IsVarArg |= VarArgIsVarArg
	}

	compileChunk(context, funcexpr.Stmts)

	context.code.AddABC(op_RETURN, 0, 1, 0, eline(funcexpr))
	context.EndScope()
	context.proto.Code = context.code.List()
	context.proto.DbgSourcePositions = context.code.PosList()
	context.proto.DbgUpvalues = context.upvalues.Names()
	context.proto.NumUpvalues = uint8(len(context.proto.DbgUpvalues))
	for _, clv := range context.proto.Constants {
		sv := ""
		if slv, ok := clv.(LString); ok {
			sv = string(slv)
		}
		context.proto.stringConstants = append(context.proto.stringConstants, sv)
	}
	patchCode(context)
} // }}}

func compileTableExpr(context *funcContext, reg int, ex *ast.TableExpr, ec *expcontext) { // {{{
	code := context.code
	/*
		tablereg := savereg(ec, reg)
		if tablereg == reg {
			reg += 1
		}
	*/
	tablereg := reg
	reg++
	code.AddABC(op_NEWTABLE, tablereg, 0, 0, sline(ex))
	tablepc := code.LastPC()
	regbase := reg

	arraycount := 0
	lastvararg := false
	for i, field := range ex.Fields {
		islast := i == len(ex.Fields)-1
		if field.Key == nil {
			if islast && isVarArgReturnExpr(field.Value) {
				reg += compileExpr(context, reg, field.Value, ecnone(-2))
				lastvararg = true
			} else {
				reg += compileExpr(context, reg, field.Value, ecnone(0))
				arraycount += 1
			}
		} else {
			regorg := reg
			b := reg
			compileExprWithKMVPropagation(context, field.Key, &reg, &b)
			c := reg
			compileExprWithKMVPropagation(context, field.Value, &reg, &c)
			opcode := op_SETTABLE
			if _, ok := field.Key.(*ast.StringExpr); ok {
				opcode = op_SETTABLEKS
			}
			code.AddABC(opcode, tablereg, b, c, sline(ex))
			reg = regorg
		}
		flush := arraycount % FieldsPerFlush
		if (arraycount != 0 && (flush == 0 || islast)) || lastvararg {
			reg = regbase
			num := flush
			if num == 0 {
				num = FieldsPerFlush
			}
			c := (arraycount-1)/FieldsPerFlush + 1
			b := num
			if islast && isVarArgReturnExpr(field.Value) {
				b = 0
			}
			line := field.Value
			if field.Key != nil {
				line = field.Key
			}
			if c > 511 {
				c = 0
			}
			code.AddABC(op_SETLIST, tablereg, b, c, sline(line))
			if c == 0 {
				code.Add(uint32(c), sline(line))
			}
		}
	}
	code.SetB(tablepc, int2Fb(arraycount))
	code.SetC(tablepc, int2Fb(len(ex.Fields)-arraycount))
	if ec.ctype == ecLocal && ec.reg != tablereg {
		code.AddABC(op_MOVE, ec.reg, tablereg, 0, sline(ex))
	}
} // }}}

func compileArithmeticOpExpr(context *funcContext, reg int, expr *ast.ArithmeticOpExpr, ec *expcontext) { // {{{
	exp := constFold(expr)
	if ex, ok := exp.(*constLValueExpr); ok {
		exp.SetLine(sline(expr))
		compileExpr(context, reg, ex, ec)
		return
	}
	expr, _ = exp.(*ast.ArithmeticOpExpr)
	a := savereg(ec, reg)
	b := reg
	compileExprWithKMVPropagation(context, expr.Lhs, &reg, &b)
	c := reg
	compileExprWithKMVPropagation(context, expr.Rhs, &reg, &c)

	op := 0
	switch expr.Operator {
	case "+":
		op = op_ADD
	case "-":
		op = op_SUB
	case "*":
		op = op_MUL
	case "/":
		op = op_DIV
	case "%":
		op = op_MOD
	case "^":
		op = op_POW
	}
	context.code.AddABC(op, a, b, c, sline(expr))
} // }}}

func compileStringConcatOpExpr(context *funcContext, reg int, expr *ast.StringConcatOpExpr, ec *expcontext) { // {{{
	code := context.code
	crange := 1
	for current := expr.Rhs; current != nil; {
		if ex, ok := current.(*ast.StringConcatOpExpr); ok {
			crange += 1
			current = ex.Rhs
		} else {
			current = nil
		}
	}
	a := savereg(ec, reg)
	basereg := reg
	reg += compileExpr(context, reg, expr.Lhs, ecnone(0))
	reg += compileExpr(context, reg, expr.Rhs, ecnone(0))
	for pc := code.LastPC(); pc != 0 && opGetOpCode(code.At(pc)) == op_CONCAT; pc-- {
		code.Pop()
	}
	code.AddABC(op_CONCAT, a, basereg, basereg+crange, sline(expr))
} // }}}

func compileUnaryOpExpr(context *funcContext, reg int, expr ast.Expr, ec *expcontext) { // {{{
	opcode := 0
	code := context.code
	var operandexpr ast.Expr
	switch ex := expr.(type) {
	case *ast.UnaryMinusOpExpr:
		exp := constFold(ex)
		if lvexpr, ok := exp.(*constLValueExpr); ok {
			exp.SetLine(sline(expr))
			compileExpr(context, reg, lvexpr, ec)
			return
		}
		ex, _ = exp.(*ast.UnaryMinusOpExpr)
		operandexpr = ex.Expr
		opcode = op_UNM
	case *ast.UnaryNotOpExpr:
		switch ex.Expr.(type) {
		case *ast.TrueExpr:
			code.AddABC(op_LOADBOOL, savereg(ec, reg), 0, 0, sline(expr))
			return
		case *ast.FalseExpr, *ast.NilExpr:
			code.AddABC(op_LOADBOOL, savereg(ec, reg), 1, 0, sline(expr))
			return
		default:
			opcode = op_NOT
			operandexpr = ex.Expr
		}
	case *ast.UnaryLenOpExpr:
		opcode = op_LEN
		operandexpr = ex.Expr
	}

	a := savereg(ec, reg)
	b := reg
	compileExprWithMVPropagation(context, operandexpr, &reg, &b)
	code.AddABC(opcode, a, b, 0, sline(expr))
} // }}}

func compileRelationalOpExprAux(context *funcContext, reg int, expr *ast.RelationalOpExpr, flip int, label int) { // {{{
	code := context.code
	b := reg
	compileExprWithKMVPropagation(context, expr.Lhs, &reg, &b)
	c := reg
	compileExprWithKMVPropagation(context, expr.Rhs, &reg, &c)
	switch expr.Operator {
	case "<":
		code.AddABC(op_LT, 0^flip, b, c, sline(expr))
	case ">":
		code.AddABC(op_LT, 0^flip, c, b, sline(expr))
	case "<=":
		code.AddABC(op_LE, 0^flip, b, c, sline(expr))
	case ">=":
		code.AddABC(op_LE, 0^flip, c, b, sline(expr))
	case "==":
		code.AddABC(op_EQ, 0^flip, b, c, sline(expr))
	case "~=":
		code.AddABC(op_EQ, 1^flip, b, c, sline(expr))
	}
	code.AddASbx(op_JMP, 0, label, sline(expr))
} // }}}

func compileRelationalOpExpr(context *funcContext, reg int, expr *ast.RelationalOpExpr, ec *expcontext) { // {{{
	a := savereg(ec, reg)
	code := context.code
	jumplabel := context.NewLabel()
	compileRelationalOpExprAux(context, reg, expr, 1, jumplabel)
	code.AddABC(op_LOADBOOL, a, 0, 1, sline(expr))
	context.SetLabelPc(jumplabel, code.LastPC())
	code.AddABC(op_LOADBOOL, a, 1, 0, sline(expr))
} // }}}

func compileLogicalOpExpr(context *funcContext, reg int, expr *ast.LogicalOpExpr, ec *expcontext) { // {{{
	a := savereg(ec, reg)
	code := context.code
	endlabel := context.NewLabel()
	lb := &lblabels{context.NewLabel(), context.NewLabel(), endlabel, false}
	nextcondlabel := context.NewLabel()
	if expr.Operator == "and" {
		compileLogicalOpExprAux(context, reg, expr.Lhs, ec, nextcondlabel, endlabel, false, lb)
		context.SetLabelPc(nextcondlabel, code.LastPC())
		compileLogicalOpExprAux(context, reg, expr.Rhs, ec, endlabel, endlabel, false, lb)
	} else {
		compileLogicalOpExprAux(context, reg, expr.Lhs, ec, endlabel, nextcondlabel, true, lb)
		context.SetLabelPc(nextcondlabel, code.LastPC())
		compileLogicalOpExprAux(context, reg, expr.Rhs, ec, endlabel, endlabel, false, lb)
	}

	if lb.b {
		context.SetLabelPc(lb.f, code.LastPC())
		code.AddABC(op_LOADBOOL, a, 0, 1, sline(expr))
		context.SetLabelPc(lb.t, code.LastPC())
		code.AddABC(op_LOADBOOL, a, 1, 0, sline(expr))
	}

	lastinst := code.Last()
	if opGetOpCode(lastinst) == op_JMP && opGetArgSbx(lastinst) == endlabel {
		code.Pop()
	}

	context.SetLabelPc(endlabel, code.LastPC())
} // }}}

func compileLogicalOpExprAux(context *funcContext, reg int, expr ast.Expr, ec *expcontext, thenlabel, elselabel int, hasnextcond bool, lb *lblabels) { // {{{
	// TODO folding constants?
	code := context.code
	flip := 0
	jumplabel := elselabel
	if hasnextcond {
		flip = 1
		jumplabel = thenlabel
	}

	switch ex := expr.(type) {
	case *ast.FalseExpr:
		if elselabel == lb.e {
			code.AddASbx(op_JMP, 0, lb.f, sline(expr))
			lb.b = true
		} else {
			code.AddASbx(op_JMP, 0, elselabel, sline(expr))
		}
		return
	case *ast.NilExpr:
		if elselabel == lb.e {
			compileExpr(context, reg, expr, ec)
			code.AddASbx(op_JMP, 0, lb.e, sline(expr))
		} else {
			code.AddASbx(op_JMP, 0, elselabel, sline(expr))
		}
		return
	case *ast.TrueExpr:
		if thenlabel == lb.e {
			code.AddASbx(op_JMP, 0, lb.t, sline(expr))
			lb.b = true
		} else {
			code.AddASbx(op_JMP, 0, thenlabel, sline(expr))
		}
		return
	case *ast.NumberExpr, *ast.StringExpr:
		if thenlabel == lb.e {
			compileExpr(context, reg, expr, ec)
			code.AddASbx(op_JMP, 0, lb.e, sline(expr))
		} else {
			code.AddASbx(op_JMP, 0, thenlabel, sline(expr))
		}
		return
	case *ast.LogicalOpExpr:
		switch ex.Operator {
		case "and":
			nextcondlabel := context.NewLabel()
			compileLogicalOpExprAux(context, reg, ex.Lhs, ec, nextcondlabel, elselabel, false, lb)
			context.SetLabelPc(nextcondlabel, context.code.LastPC())
			compileLogicalOpExprAux(context, reg, ex.Rhs, ec, thenlabel, elselabel, hasnextcond, lb)
		case "or":
			nextcondlabel := context.NewLabel()
			compileLogicalOpExprAux(context, reg, ex.Lhs, ec, thenlabel, nextcondlabel, true, lb)
			context.SetLabelPc(nextcondlabel, context.code.LastPC())
			compileLogicalOpExprAux(context, reg, ex.Rhs, ec, thenlabel, elselabel, hasnextcond, lb)
		}
		return
	case *ast.RelationalOpExpr:
		if thenlabel == elselabel {
			flip ^= 1
			jumplabel = lb.t
			lb.b = true
		} else if thenlabel == lb.e {
			jumplabel = lb.t
			lb.b = true
		} else if elselabel == lb.e {
			jumplabel = lb.f
			lb.b = true
		}
		compileRelationalOpExprAux(context, reg, ex, flip, jumplabel)
		return
	}

	if !hasnextcond && thenlabel == elselabel {
		reg += compileExpr(context, reg, expr, ec)
	} else {
		a := reg
		sreg := savereg(ec, a)
		reg += compileExpr(context, reg, expr, ecnone(0))
		if sreg == a {
			code.AddABC(op_TEST, a, 0, 0^flip, sline(expr))
		} else {
			code.AddABC(op_TESTSET, sreg, a, 0^flip, sline(expr))
		}
	}
	code.AddASbx(op_JMP, 0, jumplabel, sline(expr))
} // }}}

func compileFuncCallExpr(context *funcContext, reg int, expr *ast.FuncCallExpr, ec *expcontext) int { // {{{
	funcreg := reg
	if ec.ctype == ecLocal && ec.reg == (int(context.proto.NumParameters)-1) {
		funcreg = ec.reg
		reg = ec.reg
	}
	argc := len(expr.Args)
	islastvararg := false
	name := "(anonymous)"

	if expr.Func != nil { // hoge.func()
		reg += compileExpr(context, reg, expr.Func, ecnone(0))
		name = getExprName(context, expr.Func)
	} else { // hoge:method()
		b := reg
		compileExprWithMVPropagation(context, expr.Receiver, &reg, &b)
		c := loadRk(context, &reg, expr, LString(expr.Method))
		context.code.AddABC(op_SELF, funcreg, b, c, sline(expr))
		// increments a register for an implicit "self"
		reg = b + 1
		reg2 := funcreg + 2
		if reg2 > reg {
			reg = reg2
		}
		argc += 1
		name = string(expr.Method)
	}

	for i, ar := range expr.Args {
		islastvararg = (i == len(expr.Args)-1) && isVarArgReturnExpr(ar)
		if islastvararg {
			compileExpr(context, reg, ar, ecnone(-2))
		} else {
			reg += compileExpr(context, reg, ar, ecnone(0))
		}
	}
	b := argc + 1
	if islastvararg {
		b = 0
	}
	context.code.AddABC(op_CALL, funcreg, b, ec.varargopt+2, sline(expr))
	context.proto.DbgCalls = append(context.proto.DbgCalls, DbgCall{Pc: context.code.LastPC(), Name: name})

	if ec.varargopt == 0 && ec.ctype == ecLocal && funcreg != ec.reg {
		context.code.AddABC(op_MOVE, ec.reg, funcreg, 0, sline(expr))
		return 1
	}
	if context.RegTop() > (funcreg+2+ec.varargopt) || ec.varargopt < -1 {
		return 0
	}
	return ec.varargopt + 1
} // }}}

func loadRk(context *funcContext, reg *int, expr ast.Expr, cnst LValue) int { // {{{
	cindex := context.ConstIndex(cnst)
	if cindex <= opMaxIndexRk {
		return opRkAsk(cindex)
	} else {
		ret := *reg
		*reg++
		context.code.AddABx(op_LOADK, ret, cindex, sline(expr))
		return ret
	}
} // }}}

func getIdentRefType(context *funcContext, current *funcContext, expr *ast.IdentExpr) expContextType { // {{{
	if current == nil {
		return ecGlobal
	} else if current.FindLocalVar(expr.Value) > -1 {
		if current == context {
			return ecLocal
		}
		return ecUpvalue
	}
	return getIdentRefType(context, current.parent, expr)
} // }}}

func getExprName(context *funcContext, expr ast.Expr) string { // {{{
	switch ex := expr.(type) {
	case *ast.IdentExpr:
		return ex.Value
	case *ast.AttrGetExpr:
		switch kex := ex.Key.(type) {
		case *ast.StringExpr:
			return kex.Value
		}
		return "?"
	}
	return "?"
} // }}}

func patchCode(context *funcContext) { // {{{
	maxreg := 1
	if np := int(context.proto.NumParameters); np > 1 {
		maxreg = np
	}
	moven := 0
	code := context.code.List()
	for pc := 0; pc < len(code); pc++ {
		inst := code[pc]
		curop := opGetOpCode(inst)
		switch curop {
		case op_CLOSURE:
			pc += int(context.proto.FunctionPrototypes[opGetArgBx(inst)].NumUpvalues)
			moven = 0
			continue
		case op_SETGLOBAL, op_SETUPVAL, op_EQ, op_LT, op_LE, op_TEST,
			op_TAILCALL, op_RETURN, op_FORPREP, op_FORLOOP, op_TFORLOOP,
			op_SETLIST, op_CLOSE:
			/* nothing to do */
		case op_CALL:
			if reg := opGetArgA(inst) + opGetArgC(inst) - 2; reg > maxreg {
				maxreg = reg
			}
		case op_VARARG:
			if reg := opGetArgA(inst) + opGetArgB(inst) - 1; reg > maxreg {
				maxreg = reg
			}
		case op_SELF:
			if reg := opGetArgA(inst) + 1; reg > maxreg {
				maxreg = reg
			}
		case op_LOADNIL:
			if reg := opGetArgB(inst); reg > maxreg {
				maxreg = reg
			}
		case op_JMP: // jump to jump optimization
			distance := 0
			count := 0 // avoiding infinite loops
			for jmp := inst; opGetOpCode(jmp) == op_JMP && count < 5; jmp = context.code.At(pc + distance + 1) {
				d := context.GetLabelPc(opGetArgSbx(jmp)) - pc
				if d > opMaxArgSbx {
					if distance == 0 {
						raiseCompileError(context, context.proto.LineDefined, "too long to jump.")
					}
					break
				}
				distance = d
				count++
			}
			if distance == 0 {
				context.code.SetOpCode(pc, op_NOP)
			} else {
				context.code.SetSbx(pc, distance)
			}
		default:
			if reg := opGetArgA(inst); reg > maxreg {
				maxreg = reg
			}
		}

		// bulk move optimization(reducing op dipatch costs)
		if curop == op_MOVE {
			moven++
		} else {
			if moven > 1 {
				context.code.SetOpCode(pc-moven, op_MOVEN)
				context.code.SetC(pc-moven, intMin(moven-1, opMaxArgsC))
			}
			moven = 0
		}
	}
	maxreg++
	if maxreg > maxRegisters {
		raiseCompileError(context, context.proto.LineDefined, "register overflow(too many local variables)")
	}
	context.proto.NumUsedRegisters = uint8(maxreg)
} // }}}

func Compile(chunk []ast.Stmt, name string) (proto *FunctionProto, err error) { // {{{
	defer func() {
		if rcv := recover(); rcv != nil {
			if _, ok := rcv.(*CompileError); ok {
				err = rcv.(error)
			} else {
				panic(rcv)
			}
		}
	}()
	err = nil
	parlist := &ast.ParList{HasVargs: true, Names: []string{}}
	funcexpr := &ast.FunctionExpr{ParList: parlist, Stmts: chunk}
	context := newFuncContext(name, nil)
	compileFunctionExpr(context, funcexpr, ecnone(0))
	proto = context.proto
	return
} // }}}
