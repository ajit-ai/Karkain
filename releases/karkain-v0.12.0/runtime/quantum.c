// Karkain Quantum Runtime Library
// Phase 14: Native Quantum Circuit Execution

#include <stdio.h>
#include <stdlib.h>
#include <math.h>
#include <time.h>
#include <string.h>

#define MAX_QUBITS 10
#define MAX_STATE_SIZE (1 << MAX_QUBITS)

// Complex number structure
typedef struct {
    double real;
    double imag;
} Complex;

// Quantum register structure
typedef struct {
    int num_qubits;
    Complex state[MAX_STATE_SIZE];
    int size;
} QuantumRegister;

// Complex number operations
Complex complex_add(Complex a, Complex b) {
    return (Complex){a.real + b.real, a.imag + b.imag};
}

Complex complex_sub(Complex a, Complex b) {
    return (Complex){a.real - b.real, a.imag - b.imag};
}

Complex complex_mul(Complex a, Complex b) {
    return (Complex){a.real * b.real - a.imag * b.imag, a.real * b.imag + a.imag * b.real};
}

Complex complex_scale(Complex a, double s) {
    return (Complex){a.real * s, a.imag * s};
}

double complex_abs_sq(Complex a) {
    return a.real * a.real + a.imag * a.imag;
}

// Initialize quantum register
void qreg_init(QuantumRegister* qr, int num_qubits) {
    if (num_qubits > MAX_QUBITS) {
        printf("Error: Maximum qubits exceeded\n");
        return;
    }
    
    qr->num_qubits = num_qubits;
    qr->size = 1 << num_qubits;
    
    // Initialize to |0...0⟩ state
    for (int i = 0; i < qr->size; i++) {
        qr->state[i] = (Complex){0.0, 0.0};
    }
    qr->state[0] = (Complex){1.0, 0.0};
}

// Apply Hadamard gate to single qubit
void gate_h(QuantumRegister* qr, int target) {
    int mask = 1 << target;
    int half_size = qr->size / 2;
    
    for (int i = 0; i < half_size; i++) {
        int i0 = i;
        int i1 = i ^ mask;
        
        Complex a = qr->state[i0];
        Complex b = qr->state[i1];
        
        // H = 1/√2 * [[1, 1], [1, -1]]
        double inv_sqrt2 = 1.0 / sqrt(2.0);
        qr->state[i0] = complex_scale(complex_add(a, b), inv_sqrt2);
        qr->state[i1] = complex_scale(complex_sub(a, b), inv_sqrt2);
    }
}

// Apply Pauli-X (NOT) gate to single qubit
void gate_x(QuantumRegister* qr, int target) {
    int mask = 1 << target;
    
    for (int i = 0; i < qr->size; i++) {
        int j = i ^ mask;
        if (i < j) {
            Complex temp = qr->state[i];
            qr->state[i] = qr->state[j];
            qr->state[j] = temp;
        }
    }
}

// Apply CNOT gate
void gate_cnot(QuantumRegister* qr, int control, int target) {
    int control_mask = 1 << control;
    int target_mask = 1 << target;
    
    for (int i = 0; i < qr->size; i++) {
        if (i & control_mask) {
            int j = i ^ target_mask;
            Complex temp = qr->state[i];
            qr->state[i] = qr->state[j];
            qr->state[j] = temp;
        }
    }
}

// Measure single qubit (returns 0 or 1)
int measure(QuantumRegister* qr, int target) {
    int mask = 1 << target;
    double prob0 = 0.0;
    
    // Calculate probability of measuring 0
    for (int i = 0; i < qr->size; i++) {
        if ((i & mask) == 0) {
            prob0 += complex_abs_sq(qr->state[i]);
        }
    }
    
    // Collapse the state based on measurement
    int result;
    double r = (double)rand() / RAND_MAX;
    
    if (r < prob0) {
        result = 0;
        // Collapse to states where target qubit is 0
        double norm = sqrt(prob0);
        for (int i = 0; i < qr->size; i++) {
            if ((i & mask) == 0) {
                qr->state[i] = complex_scale(qr->state[i], 1.0 / norm);
            } else {
                qr->state[i] = (Complex){0.0, 0.0};
            }
        }
    } else {
        result = 1;
        // Collapse to states where target qubit is 1
        double norm = sqrt(1.0 - prob0);
        for (int i = 0; i < qr->size; i++) {
            if ((i & mask) != 0) {
                qr->state[i] = complex_scale(qr->state[i], 1.0 / norm);
            } else {
                qr->state[i] = (Complex){0.0, 0.0};
            }
        }
    }
    
    return result;
}

// Print quantum state (for debugging)
void qreg_print(QuantumRegister* qr) {
    printf("Quantum State (%d qubits):\n", qr->num_qubits);
    for (int i = 0; i < qr->size; i++) {
        if (complex_abs_sq(qr->state[i]) > 0.001) {
            printf("|");
            for (int j = qr->num_qubits - 1; j >= 0; j--) {
                printf("%d", (i >> j) & 1);
            }
            printf("⟩: %.3f + %.3fi\n", qr->state[i].real, qr->state[i].imag);
        }
    }
}

// Seed random number generator
void quantum_init() {
    srand(time(NULL));
}