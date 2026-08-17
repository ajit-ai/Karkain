package codegen

import (
	"fmt"
	"karkain/pkg/parser"
	"strings"
	"sync"
)

func ExecuteCPUKernelFallback(kernel *parser.KernelDeclStmt, args map[string]interface{}, globalSize int) error {
	if kernel == nil {
		return fmt.Errorf("nil kernel declaration")
	}
	_ = args

	var wg sync.WaitGroup
	errCh := make(chan error, globalSize)

	for i := 0; i < globalSize; i++ {
		wg.Add(1)
		go func(gid int) {
			defer wg.Done()
			for _, stmt := range kernel.Body {
				if _, ok := stmt.(*parser.BarrierStmt); ok {
					continue
				}
				if exprStmt, ok := stmt.(*parser.ExprStmt); ok {
					if binExpr, ok := exprStmt.Expression.(*parser.BinaryExpr); ok {
						if binExpr.Operator == "=" {
							_ = binExpr
						}
					}
				}
				_ = stmt
			}
			_ = gid
		}(i)
	}

	wg.Wait()
	close(errCh)

	for e := range errCh {
		if e != nil {
			return e
		}
	}
	return nil
}

func GenerateHostLauncher(kernel *parser.KernelDeclStmt) string {
	if kernel == nil {
		return ""
	}

	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("// Phase 18: Host launcher for GPU kernel '%s'\n", kernel.Name))
	sb.WriteString(fmt.Sprintf("static const char* %s_source = \n", kernel.Name))

	gen := NewGPUGenerator()
	openclSrc, err := gen.GenerateOpenCL(kernel)
	if err != nil {
		sb.WriteString(fmt.Sprintf("\"// Error generating OpenCL: %s\\n\"\n", err.Error()))
		return sb.String()
	}

	escaped := strings.ReplaceAll(openclSrc, "\\", "\\\\")
	escaped = strings.ReplaceAll(escaped, "\"", "\\\"")
	escaped = strings.ReplaceAll(escaped, "\n", "\\n\"\n\"")

	sb.WriteString("\"")
	sb.WriteString(escaped)
	sb.WriteString("\";\n\n")

	sb.WriteString(fmt.Sprintf("int launch_%s(", kernel.Name))
	params := []string{}
	for _, p := range kernel.Params {
		params = append(params, fmt.Sprintf("void* %s", p.Name))
	}
	sb.WriteString(strings.Join(params, ", "))
	sb.WriteString(") {\n")

	sb.WriteString("    cl_int err;\n")
	sb.WriteString("    cl_platform_id platform;\n")
	sb.WriteString("    cl_device_id device;\n")
	sb.WriteString("    cl_context context;\n")
	sb.WriteString("    cl_command_queue queue;\n")
	sb.WriteString("    cl_program program;\n")
	sb.WriteString(fmt.Sprintf("    cl_kernel cl_%s;\n\n", kernel.Name))

	sb.WriteString("    err = clGetPlatformIDs(1, &platform, NULL);\n")
	sb.WriteString("    err = clGetDeviceIDs(platform, CL_DEVICE_TYPE_DEFAULT, 1, &device, NULL);\n")
	sb.WriteString("    context = clCreateContext(NULL, 1, &device, NULL, NULL, &err);\n")
	sb.WriteString("    queue = clCreateCommandQueue(context, device, 0, &err);\n\n")

	sb.WriteString(fmt.Sprintf("    program = clCreateProgramWithSource(context, 1, &%s_source, NULL, &err);\n", kernel.Name))
	sb.WriteString("    err = clBuildProgram(program, 1, &device, NULL, NULL, NULL);\n")
	sb.WriteString(fmt.Sprintf("    cl_%s = clCreateKernel(program, \"%s\", &err);\n\n", kernel.Name, kernel.Name))

	argIdx := 0
	for _, p := range kernel.Params {
		sb.WriteString(fmt.Sprintf("    clSetKernelArg(cl_%s, %d, sizeof(cl_mem), &%s);\n", kernel.Name, argIdx, p.Name))
		argIdx++
	}

	globalSize := 256
	if kernel.WorkGroupX > 0 {
		globalSize = kernel.WorkGroupX
	}
	sb.WriteString(fmt.Sprintf("\n    size_t globalSize = %d;\n", globalSize))
	sb.WriteString(fmt.Sprintf("    err = clEnqueueNDRangeKernel(queue, cl_%s, 1, NULL, &globalSize, NULL, 0, NULL, NULL);\n", kernel.Name))
	sb.WriteString("    clFinish(queue);\n\n")

	sb.WriteString("    clReleaseKernel(cl_" + kernel.Name + ");\n")
	sb.WriteString("    clReleaseProgram(program);\n")
	sb.WriteString("    clReleaseCommandQueue(queue);\n")
	sb.WriteString("    clReleaseContext(context);\n\n")

	sb.WriteString("    return 0;\n")
	sb.WriteString("}\n")

	return sb.String()
}
