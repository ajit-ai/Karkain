package tensor

import (
	"fmt"
	"strings"
)

// LoweredOp represents a lowered tensor operation as a string
// (for integration with the SSA IR compiler).
// In a full implementation, this would emit SSA IR instructions.
type LoweredOp struct {
	FuncName string   // C runtime function name
	Args     []string // argument names
	Target   string   // output variable name
}

// LowerToC lowers a tensor graph to C runtime calls.
// Returns a list of lowered operations that can be emitted as C code.
func LowerToC(g *TensorGraph) []LoweredOp {
	var ops []LoweredOp
	for _, node := range g.Nodes {
		op := lowerNode(node)
		if op != nil {
			ops = append(ops, *op)
		}
	}
	return ops
}

func lowerNode(n *TensorNode) *LoweredOp {
	switch n.Op {
	case OpCreate:
		return &LoweredOp{
			FuncName: "tensor_create",
			Args:     []string{fmt.Sprintf("%d", n.Shape.Rank()), shapeStr(n.Shape), string(rune(n.DType))},
			Target:   n.ID,
		}
	case OpAdd:
		return &LoweredOp{
			FuncName: "tensor_add",
			Args:     argIDs(n),
			Target:   n.ID,
		}
	case OpSub:
		return &LoweredOp{
			FuncName: "tensor_sub",
			Args:     argIDs(n),
			Target:   n.ID,
		}
	case OpMul:
		return &LoweredOp{
			FuncName: "tensor_mul",
			Args:     argIDs(n),
			Target:   n.ID,
		}
	case OpDiv:
		return &LoweredOp{
			FuncName: "tensor_div",
			Args:     argIDs(n),
			Target:   n.ID,
		}
	case OpMatMul:
		return &LoweredOp{
			FuncName: "tensor_matmul",
			Args:     argIDs(n),
			Target:   n.ID,
		}
	case OpRelu:
		return &LoweredOp{
			FuncName: "tensor_relu",
			Args:     argIDs(n),
			Target:   n.ID,
		}
	case OpSigmoid:
		return &LoweredOp{
			FuncName: "tensor_sigmoid",
			Args:     argIDs(n),
			Target:   n.ID,
		}
	case OpTanh:
		return &LoweredOp{
			FuncName: "tensor_tanh",
			Args:     argIDs(n),
			Target:   n.ID,
		}
	case OpSoftmax:
		return &LoweredOp{
			FuncName: "tensor_softmax",
			Args:     argIDs(n),
			Target:   n.ID,
		}
	case OpReshape:
		return &LoweredOp{
			FuncName: "tensor_reshape",
			Args:     append(argIDs(n), shapeStr(n.Shape)),
			Target:   n.ID,
		}
	case OpTranspose:
		return &LoweredOp{
			FuncName: "tensor_transpose",
			Args:     argIDs(n),
			Target:   n.ID,
		}
	case OpReduceSum:
		return &LoweredOp{
			FuncName: "tensor_reduce_sum",
			Args:     argIDs(n),
			Target:   n.ID,
		}
	case OpCopy:
		return &LoweredOp{
			FuncName: "tensor_copy",
			Args:     argIDs(n),
			Target:   n.ID,
		}
	}
	return nil
}

func argIDs(n *TensorNode) []string {
	ids := make([]string, 0, len(n.Args))
	for _, arg := range n.Args {
		if s, ok := arg.(string); ok {
			ids = append(ids, s)
		}
	}
	return ids
}

func shapeStr(s Shape) string {
	if s == nil || len(s) == 0 {
		return "nil"
	}
	parts := make([]string, len(s))
	for i, d := range s {
		parts[i] = d.String()
	}
	return "[" + strings.Join(parts, ",") + "]"
}

// EmitC generates C23 code for a lowered operation.
func EmitC(op LoweredOp) string {
	if len(op.Args) == 0 {
		return fmt.Sprintf("Tensor* %s = %s();", op.Target, op.FuncName)
	}
	return fmt.Sprintf("Tensor* %s = %s(%s);", op.Target, op.FuncName, strings.Join(op.Args, ", "))
}

// EmitRuntimeHeader generates the C runtime header for tensor operations.
func EmitRuntimeHeader() string {
	return `
/* Karkain Tensor IR Runtime */
typedef struct Tensor {
    int ndim;
    int* shape;
    int* strides;
    int dtype;
    void* data;
    int refcount;
} Tensor;

Tensor* tensor_create(int ndim, int* shape, int dtype);
void tensor_destroy(Tensor* t);
Tensor* tensor_add(Tensor* a, Tensor* b);
Tensor* tensor_sub(Tensor* a, Tensor* b);
Tensor* tensor_mul(Tensor* a, Tensor* b);
Tensor* tensor_div(Tensor* a, Tensor* b);
Tensor* tensor_matmul(Tensor* a, Tensor* b);
Tensor* tensor_relu(Tensor* x);
Tensor* tensor_sigmoid(Tensor* x);
Tensor* tensor_tanh(Tensor* x);
Tensor* tensor_softmax(Tensor* x, int axis);
Tensor* tensor_reshape(Tensor* t, int* new_shape);
Tensor* tensor_transpose(Tensor* t);
Tensor* tensor_reduce_sum(Tensor* t, int axis);
Tensor* tensor_copy(Tensor* t);
`
}
