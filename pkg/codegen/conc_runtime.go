package codegen

import (
	"fmt"
	"strings"

	"karkain/pkg/parser"
	"karkain/runtime/concurrency"
)

// Concurrent builtins dispatched from CallExpr.
func concBuiltin(fn string) bool {
	switch fn {
	case "channel", "send", "chanSend", "chanClose", "join", "wait_all",
		"actor", "actorSend", "actorState", "setActorState", "actorStop":
		return true
	}
	return false
}

// scanNodeForConcurrency walks an AST node and records everything the Phase 107
// codegen needs to know: whether any concurrency construct is used (drives
// runtime embedding + main() init/finish hooks), which user functions are
// spawned (per-site wrappers) and which handler names are referenced by
// actor(...) (dispatcher switch cases). Mirrors scanNodeForHTTP's traversal.
func (g *Generator) scanNodeForConcurrency(node parser.Node) {
	if node == nil {
		return
	}
	switch n := node.(type) {
	case *parser.SpawnExpr:
		g.usesConcurrency = true
		for _, a := range n.Args {
			g.scanNodeForConcurrency(a)
		}
	case *parser.ChRecvExpr:
		g.usesConcurrency = true
		g.scanNodeForConcurrency(n.Channel)
	case *parser.SendExpr:
		g.usesConcurrency = true
		g.scanNodeForConcurrency(n.Channel)
		g.scanNodeForConcurrency(n.Message)
	case *parser.ChSendExpr:
		g.usesConcurrency = true
		g.scanNodeForConcurrency(n.Channel)
		g.scanNodeForConcurrency(n.Value)
	case *parser.ReceiveStmt:
		g.usesConcurrency = true
		g.scanNodeForConcurrency(n.Channel)
	case *parser.CallExpr:
		if concBuiltin(n.Function) {
			g.usesConcurrency = true
		}
		for _, a := range n.Args {
			g.scanNodeForConcurrency(a)
		}
	case *parser.IndirectCallExpr:
		// Phase 133: concurrency builtins may hide in callee/args.
		g.scanNodeForConcurrency(n.Target)
		for _, a := range n.Args {
			g.scanNodeForConcurrency(a)
		}
	case *parser.FuncDecl:
		for _, s := range n.Body {
			g.scanNodeForConcurrency(s)
		}
	case *parser.IfStmt:
		g.scanNodeForConcurrency(n.Condition)
		for _, s := range n.Consequence {
			g.scanNodeForConcurrency(s)
		}
		for _, s := range n.Alternative {
			g.scanNodeForConcurrency(s)
		}
	case *parser.WhileStmt:
		g.scanNodeForConcurrency(n.Condition)
		for _, s := range n.Body {
			g.scanNodeForConcurrency(s)
		}
	case *parser.ForStmt:
		g.scanNodeForConcurrency(n.Init)
		g.scanNodeForConcurrency(n.Condition)
		g.scanNodeForConcurrency(n.Post)
		for _, s := range n.Body {
			g.scanNodeForConcurrency(s)
		}
	case *parser.ForInStmt:
		g.scanNodeForConcurrency(n.Iter)
		for _, s := range n.Body {
			g.scanNodeForConcurrency(s)
		}
	case *parser.VarDeclStmt:
		g.scanNodeForConcurrency(n.Value)
	case *parser.ReturnStmt:
		g.scanNodeForConcurrency(n.Value)
	case *parser.ExprStmt:
		g.scanNodeForConcurrency(n.Expression)
	case *parser.PrintStmt:
		g.scanNodeForConcurrency(n.Value)
	case *parser.BinaryExpr:
		g.scanNodeForConcurrency(n.Left)
		g.scanNodeForConcurrency(n.Right)
	case *parser.UnaryExpr:
		g.scanNodeForConcurrency(n.Operand)
	case *parser.LambdaExpr:
		for _, s := range n.Body {
			g.scanNodeForConcurrency(s)
		}
	case *parser.MatchExpr:
		g.scanNodeForConcurrency(n.Value)
		for _, arm := range n.Arms {
			g.scanNodeForConcurrency(arm.Body)
		}
	case *parser.IndexExpr:
		g.scanNodeForConcurrency(n.Left)
		g.scanNodeForConcurrency(n.Index)
	case *parser.SliceExpr:
		g.scanNodeForConcurrency(n.Target)
		g.scanNodeForConcurrency(n.Start)
		g.scanNodeForConcurrency(n.End)
	case *parser.DotExpr:
		g.scanNodeForConcurrency(n.Left)
	case *parser.ArrayLiteral:
		for _, e := range n.Elements {
			g.scanNodeForConcurrency(e)
		}
	case *parser.MapLiteral:
		for _, k := range n.Keys {
			g.scanNodeForConcurrency(k)
		}
		for _, v := range n.Values {
			g.scanNodeForConcurrency(v)
		}
	case *parser.StructLiteral:
		for _, f := range n.Fields {
			g.scanNodeForConcurrency(f)
		}
	}
}

// collectConcDecls records the spawn targets and actor handler names so the
// wrapper prepass can be emitted. It mirrors scanNodeForConcurrency's
// traversal so spawn/actor sites nested inside function bodies are found.
func (g *Generator) collectConcDecls(node parser.Node) {
	if node == nil {
		return
	}
	switch n := node.(type) {
	case *parser.FuncDecl:
		for _, s := range n.Body {
			g.collectConcDecls(s)
		}
	case *parser.IfStmt:
		g.collectConcDecls(n.Condition)
		for _, s := range n.Consequence {
			g.collectConcDecls(s)
		}
		for _, s := range n.Alternative {
			g.collectConcDecls(s)
		}
	case *parser.WhileStmt:
		g.collectConcDecls(n.Condition)
		for _, s := range n.Body {
			g.collectConcDecls(s)
		}
	case *parser.ForStmt:
		g.collectConcDecls(n.Init)
		g.collectConcDecls(n.Condition)
		g.collectConcDecls(n.Post)
		for _, s := range n.Body {
			g.collectConcDecls(s)
		}
	case *parser.ForInStmt:
		g.collectConcDecls(n.Iter)
		for _, s := range n.Body {
			g.collectConcDecls(s)
		}
	case *parser.VarDeclStmt:
		g.collectConcDecls(n.Value)
	case *parser.ReturnStmt:
		g.collectConcDecls(n.Value)
	case *parser.ExprStmt:
		g.collectConcDecls(n.Expression)
	case *parser.PrintStmt:
		g.collectConcDecls(n.Value)
	case *parser.BinaryExpr:
		g.collectConcDecls(n.Left)
		g.collectConcDecls(n.Right)
	case *parser.UnaryExpr:
		g.collectConcDecls(n.Operand)
	case *parser.LambdaExpr:
		for _, s := range n.Body {
			g.collectConcDecls(s)
		}
	case *parser.MatchExpr:
		g.collectConcDecls(n.Value)
		for _, arm := range n.Arms {
			g.collectConcDecls(arm.Body)
		}
	case *parser.IndexExpr:
		g.collectConcDecls(n.Left)
		g.collectConcDecls(n.Index)
	case *parser.SliceExpr:
		g.collectConcDecls(n.Target)
		g.collectConcDecls(n.Start)
		g.collectConcDecls(n.End)
	case *parser.DotExpr:
		g.collectConcDecls(n.Left)
	case *parser.ArrayLiteral:
		for _, e := range n.Elements {
			g.collectConcDecls(e)
		}
	case *parser.MapLiteral:
		for _, k := range n.Keys {
			g.collectConcDecls(k)
		}
		for _, v := range n.Values {
			g.collectConcDecls(v)
		}
	case *parser.StructLiteral:
		for _, f := range n.Fields {
			g.collectConcDecls(f)
		}
	case *parser.IndirectCallExpr:
		// Phase 133: actor declarations may hide in callee/args.
		g.collectConcDecls(n.Target)
		for _, a := range n.Args {
			g.collectConcDecls(a)
		}
	case *parser.CallExpr:
		if n.Function == "actor" && len(n.Args) > 0 {
			if s, ok := n.Args[0].(*parser.StringLiteral); ok && s.Value != "" {
				if !g.concHandlers[s.Value] {
					g.concHandlers[s.Value] = true
					g.concHandlerOrder = append(g.concHandlerOrder, s.Value)
				}
			}
		}
		for _, a := range n.Args {
			g.collectConcDecls(a)
		}
	case *parser.SpawnExpr:
		if n.ActorName != "" && !g.concSpawned[n.ActorName] {
			g.concSpawned[n.ActorName] = true
			g.concSpawnOrder = append(g.concSpawnOrder, n.ActorName)
		}
		for _, a := range n.Args {
			g.collectConcDecls(a)
		}
	case *parser.ChRecvExpr:
		g.collectConcDecls(n.Channel)
	case *parser.SendExpr:
		g.collectConcDecls(n.Channel)
		g.collectConcDecls(n.Message)
	case *parser.ChSendExpr:
		g.collectConcDecls(n.Channel)
		g.collectConcDecls(n.Value)
	case *parser.ReceiveStmt:
		g.collectConcDecls(n.Channel)
	}
}

// stripEmbedInclude removes the runtime's self-includes (`#include
// "karkain_conc.h"`, `#include "karkain_sched_impl.h"`) so the concatenated
// bodies compile once inside the generated translation unit, where the header
// text has already been emitted verbatim by concRuntimeC.
func stripEmbedInclude(src string) string {
	lines := strings.Split(src, "\n")
	out := make([]string, 0, len(lines))
	for _, ln := range lines {
		t := strings.TrimSpace(ln)
		if strings.HasPrefix(t, `#include "karkain_conc.h"`) ||
			strings.HasPrefix(t, `#include "karkain_sched_impl.h"`) {
			continue
		}
		out = append(out, ln)
	}
	return strings.Join(out, "\n")
}

// concRuntimeHeader emits the compiler-neutral concurrency runtime core: the
// public header, the internal shared header and the three implementation
// translation units, concatenated into a single translation-unit-shaped block.
// Each file's self-include is stripped; the header guard keeps the whole thing
// idempotent. This is the exact source the pkg/runtime gate compiles standalone.
func concRuntimeHeader() string {
	var b strings.Builder
	for _, f := range concurrency.Files {
		b.WriteString("// ==== Phase 107 runtime source: " + f.Name + " ====\n")
		b.WriteString(stripEmbedInclude(f.Src))
		b.WriteString("\n")
	}
	return b.String()
}

// concWrapperC emits the compile-time wrapper machinery for one program:
//   - karkain_actor_box_t (payload + handler id) and the actor dispatcher
//     switch, invoked by the Value glue when an actor drains a message;
//   - one static task-wrapper per spawn site (karkain_run_0, karkain_run_1,
//     ...) that unpacks the heap Value array ctx, calls the user function with
//     the exact arity and records the task status from its integer result.
//
// It is emitted after the user function forward declarations so every callee
// symbol is declared before the wrappers reference it.
func (g *Generator) concWrapperC() string {
	var b strings.Builder

	b.WriteString(`
// ============================================================
// Phase 107: per-program concurrency wrappers
// ============================================================
typedef struct { long bid; Value state; } karkain_actor_box_t;
static void karkain_actor_dispatch(karkain_actor_t* karkain_conc_a, void* karkain_conc_m,
                                   karkain_sched_t* karkain_conc_s) {
    (void)karkain_conc_s;
    Value karkain_conc_msg = *((Value*)karkain_conc_m);
    free(karkain_conc_m);
    karkain_actor_box_t* karkain_conc_b = (karkain_actor_box_t*)karkain_conc_a->udata;
    switch (karkain_conc_b->bid) {
`)
	for i, h := range g.concHandlerOrder {
		fmt.Fprintf(&b, "    case %d: {\n", i)
		fmt.Fprintf(&b, "        Value _ns = %s(karkain_conc_b->state, karkain_conc_msg);\n",
			userFuncC(h))
		fmt.Fprintf(&b,
			"        /* adopt the returned state; int 0 (fallthrough) means no change */\n")
		fmt.Fprintf(&b,
			"        if (!(_ns.type == TYPE_INT && _ns.intVal == 0)) karkain_conc_b->state = _ns;\n")
		fmt.Fprintf(&b, "        break;\n    }\n")
	}
	b.WriteString("    default: break;\n    }\n}\n\n")

	for i, fn := range g.concSpawnOrder {
		fmt.Fprintf(&b, "static void karkain_run_%d(karkain_task_t* karkain_conc_t, void* karkain_conc_ctx,\n",
			i)
		b.WriteString("                         karkain_sched_t* karkain_conc_s) {\n")
		b.WriteString("    (void)karkain_conc_s;\n")
		b.WriteString("    Value* karkain_conc_args = (Value*)karkain_conc_ctx;\n")
		var call strings.Builder
		call.WriteString(userFuncC(fn))
		call.WriteString("(")
		arity := 0
		if f, ok := g.concFns[fn]; ok {
			arity = len(f.Params)
		}
		for i := 0; i < arity; i++ {
			if i > 0 {
				call.WriteString(", ")
			}
			fmt.Fprintf(&call, "karkain_conc_args[%d]", i)
		}
		call.WriteString(")")
		fmt.Fprintf(&b, "    Value _r = %s;\n", call.String())
		fmt.Fprintf(&b, "    %s\n", concTaskStatusOf("_r"))
		b.WriteString("}\n\n")
	}
	return b.String()
}

// concRuntimeAPIC emits the Value-level glue layer bridging the compiler-neutral
// runtime (opaque void* payloads, intptr handles) and the codegen Value model.
// Every Karkain concurrency builtin lowers to one of these functions. All
// values cross the boundary as intptr-typed handles in Value.intVal (tasks,
// channels, actors) or as heap-owned boxes for payloads and actor state.
func concRuntimeAPIC() string {
	return `
// ============================================================
// Phase 107: Karkain concurrency API glue (Value model bridge)
// ============================================================
static karkain_sched_t* karkain_conc_sched_handle(void) {
    return karkain_conc_init();
}

static Value karkain_conc_mk_handle(void* p) {
    Value v; v.type = TYPE_INT; v.intVal = (long long)(intptr_t)p; return v;
}
static void* karkain_conc_handle_ptr(Value v) {
    return (void*)(intptr_t)v.intVal;
}

/* channel(cap): cap < 0 -> unbounded, cap >= 0 -> bounded. channel() (no arg)
 * is emitted as make_int(-1) by genConcCall. */
static Value karkain_conc_channel(Value cap) {
    long n = (cap.type == TYPE_INT && cap.intVal < 0) ? -1 : (long)cap.intVal;
    karkain_channel_t* c = karkain_channel_create(karkain_conc_sched_handle(), n);
    return karkain_conc_mk_handle(c);
}

/* send(ch, v) / chanSend(ch, v): copies v into a heap box owned by the
 * channel; receiver unboxes and frees. Returns 1 sent / 0 rejected (closed). */
static Value karkain_conc_send(Value ch, Value msg) {
    Value* box = (Value*)malloc(sizeof(Value));
    if (!box) return make_int(0);
    *box = msg;
    int r = karkain_channel_send((karkain_channel_t*)karkain_conc_handle_ptr(ch), box);
    return make_int(r);
}

/* receive(ch): blocking; returns the message Value, or nil when closed+drained. */
static Value karkain_conc_recv(Value ch) {
    void* m = karkain_channel_recv((karkain_channel_t*)karkain_conc_handle_ptr(ch));
    if (!m) return mk_nil();
    Value v = *(Value*)m;
    free(m);
    return v;
}

/* chanClose(ch): closea a channel; returns 1 on first close, 0 when already. */
static Value karkain_conc_close(Value ch) {
    int r = karkain_channel_close((karkain_channel_t*)karkain_conc_handle_ptr(ch));
    return make_int(r);
}

/* join(t): blocks until the task completes, returns its status (0 ok, <0 failed). */
static Value karkain_conc_join(Value t) {
    long st = karkain_task_join((karkain_task_t*)karkain_conc_handle_ptr(t));
    return make_int(st);
}

static void karkain_conc_wait_all(void) { /* Value-wrapper: usable as expression */
    karkain_wait_all(karkain_conc_sched_handle());
}

/* actor(handlerId, state): state is boxed into a heap cell alongside the
 * handler id; the generated dispatcher switches on the id. Returns the actor. */
static Value karkain_conc_actor(long handlerId, Value state) {
    karkain_actor_box_t* box = (karkain_actor_box_t*)malloc(sizeof(karkain_actor_box_t));
    if (!box) return make_int(0);
    box->bid = handlerId;
    box->state = state;
    karkain_actor_t* a = karkain_actor_create(karkain_conc_sched_handle(),
                                              karkain_actor_dispatch, box);
    if (!a) { free(box); return make_int(0); }
    return karkain_conc_mk_handle(a);
}

/* actorSend(a, msg): box a Value copy; returns 1 accepted / 0 if stopped. */
static Value karkain_conc_actor_send(Value a, Value msg) {
    Value* box = (Value*)malloc(sizeof(Value));
    if (!box) return make_int(0);
    *box = msg;
    int r = karkain_actor_send((karkain_actor_t*)karkain_conc_handle_ptr(a), box);
    return make_int(r);
}

static Value karkain_conc_actor_state(Value a) {
    karkain_actor_box_t* b = (karkain_actor_box_t*)karkain_actor_get_state(
        (karkain_actor_t*)karkain_conc_handle_ptr(a));
    return b ? b->state : mk_nil();
}

static void karkain_conc_actor_set_state(Value a, Value v) {
    karkain_actor_box_t* b = (karkain_actor_box_t*)karkain_actor_get_state(
        (karkain_actor_t*)karkain_conc_handle_ptr(a));
    if (b) b->state = v;
}

static void karkain_conc_actor_stop(Value a) {
    karkain_actor_stop((karkain_actor_t*)karkain_conc_handle_ptr(a));
}
`
}

// genConcCall lowers a concurrency builtin CallExpr to its Value-glue call.
// Callers pass the already-generated argument C expressions; the handler-id
// resolution for actor(...) happens here (the handler name is a string literal).
func (g *Generator) genConcCall(n *parser.CallExpr, args []string) string {
	switch n.Function {
	case "channel":
		if len(args) == 0 {
			return "karkain_conc_channel(make_int(-1))"
		}
		return fmt.Sprintf("karkain_conc_channel(%s)", args[0])
	case "send", "chanSend":
		return fmt.Sprintf("karkain_conc_send(%s, %s)", args[0], args[1])
	case "chanClose":
		return fmt.Sprintf("karkain_conc_close(%s)", args[0])
	case "join":
		return fmt.Sprintf("karkain_conc_join(%s)", args[0])
	case "wait_all":
		return "(karkain_conc_wait_all(), mk_nil())"
	case "actor":
		idx := -1
		if sl, ok := n.Args[0].(*parser.StringLiteral); ok {
			for i, h := range g.concHandlerOrder {
				if h == sl.Value {
					idx = i
					break
				}
			}
		}
		if idx < 0 {
			return "mk_nil() /* unknown actor handler */"
		}
		return fmt.Sprintf("karkain_conc_actor(%d, %s)", idx, args[1])
	case "actorSend":
		return fmt.Sprintf("karkain_conc_actor_send(%s, %s)", args[0], args[1])
	case "actorState":
		return fmt.Sprintf("karkain_conc_actor_state(%s)", args[0])
	case "setActorState":
		return fmt.Sprintf("(karkain_conc_actor_set_state(%s, %s), mk_nil())", args[0], args[1])
	case "actorStop":
		return fmt.Sprintf("(karkain_conc_actor_stop(%s), mk_nil())", args[0])
	}
	return "mk_nil()"
}

// concSpawnSiteIndex maps a spawned user function name to its wrapper index
// (karkain_run_<idx>), or -1 when the name was not collected by the scan.
func concSpawnSiteIndex(order []string, fn string) int {
	for i, f := range order {
		if f == fn {
			return i
		}
	}
	return -1
}

// genConcurrencySpawnExpr lowers a SpawnExpr to a spawn call. The user-function
// wrapper (karkain_run_<idx>) carries the function pointer; the argument
// values are copied into a heap-owned Value array that the runtime frees after
// the task completes. Returns a numeric handle Value.
func (g *Generator) genConcurrencySpawn(n *parser.SpawnExpr) string {
	idx := concSpawnSiteIndex(g.concSpawnOrder, n.ActorName)
	if idx < 0 {
		return "mk_nil() /* unknown spawn target */"
	}
	args := make([]string, 0, len(n.Args))
	for _, a := range n.Args {
		args = append(args, g.genExpr(a))
	}
	if len(args) == 0 {
		return fmt.Sprintf("karkain_conc_mk_handle(karkain_sched_spawn(karkain_conc_sched_handle(), karkain_run_%d, NULL))", idx)
	}
	vals := strings.Join(args, ", ")
	return fmt.Sprintf(
		"({ Value _conc_sa[%d] = {%s}; long _i; Value* _conc_h = (Value*)malloc(%d * sizeof(Value)); for (_i = 0; _i < %d; _i++) _conc_h[_i] = _conc_sa[_i]; karkain_conc_mk_handle(karkain_sched_spawn(karkain_conc_sched_handle(), karkain_run_%d, _conc_h)); })",
		len(args), vals, len(args), len(args), idx)
}

// genConcRecv lowers a ChRecvExpr (receive(ch) as an expression).
func (g *Generator) genConcRecv(n *parser.ChRecvExpr) string {
	return fmt.Sprintf("karkain_conc_recv(%s)", g.genExpr(n.Channel))
}

func concTaskStatusOf(expr string) string {
	return fmt.Sprintf(
		"(karkain_conc_t->status = ((%[1]s).type == TYPE_INT ? (%[1]s).intVal : 0));", expr)
}