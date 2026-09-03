package tensor

import "fmt"

// Op represents a tensor operation.
type Op int

const (
	// Creation
	OpCreate Op = iota // allocate tensor with shape
	OpConstant        // constant tensor from values

	// Element access
	OpLoad  // read element
	OpStore // write element

	// Element-wise arithmetic
	OpAdd
	OpSub
	OpMul
	OpDiv
	OpMod

	// Matrix operations
	OpMatMul // matrix multiply

	// Shape manipulation
	OpReshape    // change shape
	OpTranspose  // permute dimensions
	OpBroadcast  // expand dimensions
	OpSlice      // extract sub-tensor
	OpConcat     // concatenate tensors
	OpReduceSum  // reduce along axis
	OpReduceMean // reduce along axis

	// Activation functions
	OpRelu
	OpSigmoid
	OpTanh
	OpSoftmax

	// Comparison
	OpEqual
	OpNotEqual
	OpLess
	OpGreater

	// Logical
	OpAnd
	OpOr
	OpNot

	// Utility
	OpCopy   // deep copy
	OpReshape_
)

// String returns the name of the operation.
func (o Op) String() string {
	switch o {
	case OpCreate:
		return "create"
	case OpConstant:
		return "constant"
	case OpLoad:
		return "load"
	case OpStore:
		return "store"
	case OpAdd:
		return "add"
	case OpSub:
		return "sub"
	case OpMul:
		return "mul"
	case OpDiv:
		return "div"
	case OpMod:
		return "mod"
	case OpMatMul:
		return "matmul"
	case OpReshape:
		return "reshape"
	case OpReshape_:
		return "reshape"
	case OpTranspose:
		return "transpose"
	case OpBroadcast:
		return "broadcast"
	case OpSlice:
		return "slice"
	case OpConcat:
		return "concat"
	case OpReduceSum:
		return "reduce_sum"
	case OpReduceMean:
		return "reduce_mean"
	case OpRelu:
		return "relu"
	case OpSigmoid:
		return "sigmoid"
	case OpTanh:
		return "tanh"
	case OpSoftmax:
		return "softmax"
	case OpEqual:
		return "equal"
	case OpNotEqual:
		return "not_equal"
	case OpLess:
		return "less"
	case OpGreater:
		return "greater"
	case OpAnd:
		return "and"
	case OpOr:
		return "or"
	case OpNot:
		return "not"
	case OpCopy:
		return "copy"
	default:
		return "unknown"
	}
}

// NumInputs returns the expected number of inputs for an operation.
func (o Op) NumInputs() int {
	switch o {
	case OpCreate, OpConstant, OpLoad, OpCopy:
		return 0
	case OpStore:
		return 2 // tensor, value
	case OpAdd, OpSub, OpMul, OpDiv, OpMod:
		return 2
	case OpMatMul:
		return 2
	case OpReshape, OpReshape_:
		return 1
	case OpTranspose:
		return 1
	case OpBroadcast:
		return 1
	case OpSlice:
		return 1
	case OpConcat:
		return -1 // variadic
	case OpReduceSum, OpReduceMean:
		return 1
	case OpRelu, OpSigmoid, OpTanh:
		return 1
	case OpSoftmax:
		return 1
	case OpEqual, OpNotEqual, OpLess, OpGreater:
		return 2
	case OpAnd, OpOr:
		return 2
	case OpNot:
		return 1
	default:
		return 0
	}
}

// IsElementWise returns true if the operation is element-wise.
func (o Op) IsElementWise() bool {
	switch o {
	case OpAdd, OpSub, OpMul, OpDiv, OpMod:
		return true
	case OpRelu, OpSigmoid, OpTanh:
		return true
	case OpEqual, OpNotEqual, OpLess, OpGreater:
		return true
	case OpAnd, OpOr, OpNot:
		return true
	}
	return false
}

// ============================================================
// Shape Rules
// ============================================================

// ShapeRule computes the output shape of an operation given input shapes.
type ShapeRule func(inputs []Shape) (Shape, error)

// GetShapeRule returns the shape rule for an operation.
func GetShapeRule(op Op) ShapeRule {
	switch op {
	case OpAdd, OpSub, OpMul, OpDiv, OpMod:
		return binaryShapeRule
	case OpMatMul:
		return matmulShapeRule
	case OpRelu, OpSigmoid, OpTanh, OpNot:
		return unaryShapeRule
	case OpSoftmax:
		return unaryShapeRule
	case OpEqual, OpNotEqual, OpLess, OpGreater:
		return binaryShapeRuleBool
	case OpAnd, OpOr:
		return binaryShapeRuleBool
	case OpReduceSum, OpReduceMean:
		return reduceShapeRule
	case OpTranspose:
		return transposeShapeRule
	default:
		return nil
	}
}

func binaryShapeRule(inputs []Shape) (Shape, error) {
	if len(inputs) != 2 {
		return nil, opError("binary", 2, len(inputs))
	}
	return broadcastShapes(inputs[0], inputs[1])
}

func binaryShapeRuleBool(inputs []Shape) (Shape, error) {
	if len(inputs) != 2 {
		return nil, opError("binary", 2, len(inputs))
	}
	return broadcastShapes(inputs[0], inputs[1])
}

func matmulShapeRule(inputs []Shape) (Shape, error) {
	if len(inputs) != 2 {
		return nil, opError("matmul", 2, len(inputs))
	}
	a, b := inputs[0], inputs[1]
	if a.Rank() < 2 || b.Rank() < 2 {
		return nil, tensorError("matmul requires rank >= 2 inputs")
	}
	// Last dim of a must equal second-to-last dim of b
	aLast := a[a.Rank()-1]
	bSecondLast := b[b.Rank()-2]
	if aLast.IsStatic() && bSecondLast.IsStatic() && aLast.Size != bSecondLast.Size {
		return nil, tensorError("matmul inner dimensions mismatch: %d != %d", aLast.Size, bSecondLast.Size)
	}
	// Batch dims: broadcast a's batch dims with b's batch dims
	// For simplicity, align from left and check compatibility
	aBatch := a[:a.Rank()-2]
	bBatch := b[:b.Rank()-2]
	batchShape, err := broadcastShapes(aBatch, bBatch)
	if err != nil {
		return nil, tensorError("matmul batch dimension error: %v", err)
	}
	// Output: batch_dims + [a_rows, b_cols]
	out := make(Shape, 0, batchShape.Rank()+2)
	out = append(out, batchShape...)
	out = append(out, a[a.Rank()-2]) // a rows
	out = append(out, b[b.Rank()-1]) // b cols
	return out, nil
}

func unaryShapeRule(inputs []Shape) (Shape, error) {
	if len(inputs) != 1 {
		return nil, opError("unary", 1, len(inputs))
	}
	return inputs[0], nil
}

func reduceShapeRule(inputs []Shape) (Shape, error) {
	if len(inputs) != 1 {
		return nil, opError("reduce", 1, len(inputs))
	}
	// Reduce removes the last dimension
	in := inputs[0]
	if in.Rank() == 0 {
		return nil, tensorError("cannot reduce scalar")
	}
	out := make(Shape, in.Rank()-1)
	copy(out, in[:in.Rank()-1])
	return out, nil
}

func transposeShapeRule(inputs []Shape) (Shape, error) {
	if len(inputs) != 1 {
		return nil, opError("transpose", 1, len(inputs))
	}
	// Reverse dimensions
	in := inputs[0]
	out := make(Shape, in.Rank())
	for i := 0; i < in.Rank(); i++ {
		out[i] = in[in.Rank()-1-i]
	}
	return out, nil
}

// ============================================================
// Broadcasting (Karkain-defined rules)
// ============================================================

// broadcastShapes computes the output shape from two input shapes.
// Rules:
//  1. Dimensions align from the right
//  2. Missing dimensions are treated as 1
//  3. Size 1 dimensions broadcast to match
//  4. Mismatched sizes > 1 is an error
func broadcastShapes(a, b Shape) (Shape, error) {
	rankA := a.Rank()
	rankB := b.Rank()
	maxRank := rankA
	if rankB > maxRank {
		maxRank = rankB
	}

	out := make(Shape, maxRank)
	for i := 0; i < maxRank; i++ {
		// Index from the right
		idxA := rankA - 1 - i
		idxB := rankB - 1 - i

		var dimA, dimB Dimension
		if idxA >= 0 {
			dimA = a[idxA]
		} else {
			dimA = StaticDim(1)
		}
		if idxB >= 0 {
			dimB = b[idxB]
		} else {
			dimB = StaticDim(1)
		}

		// Both static: check compatibility
		if dimA.IsStatic() && dimB.IsStatic() {
			if dimA.Size == dimB.Size {
				out[maxRank-1-i] = StaticDim(dimA.Size)
			} else if dimA.Size == 1 {
				out[maxRank-1-i] = StaticDim(dimB.Size)
			} else if dimB.Size == 1 {
				out[maxRank-1-i] = StaticDim(dimA.Size)
			} else {
				return nil, tensorError("broadcasting dimensions %d and %d are incompatible", dimA.Size, dimB.Size)
			}
		} else if dimA.IsStatic() && dimA.Size == 1 {
			out[maxRank-1-i] = dimB
		} else if dimB.IsStatic() && dimB.Size == 1 {
			out[maxRank-1-i] = dimA
		} else if dimA.Kind == dimB.Kind && dimA.Size == dimB.Size && dimA.Name == dimB.Name {
			out[maxRank-1-i] = dimA
		} else {
			// Dynamic or symbolic: keep as dynamic
			out[maxRank-1-i] = DynamicDim()
		}
	}

	return out, nil
}

// ============================================================
// Helpers
// ============================================================

func opError(name string, expected, got int) error {
	return fmt.Errorf("%s operation requires %d inputs, got %d", name, expected, got)
}

func tensorError(msg string, args ...interface{}) error {
	return fmt.Errorf(msg, args...)
}
