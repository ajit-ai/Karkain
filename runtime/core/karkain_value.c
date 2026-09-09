#include "karkain_value.h"

#include "karkain_mem.h"
#include "karkain_runtime.h"

Value make_int(long long v) {
    Value val;
    val.type = TYPE_INT;
    val.intVal = v;
    return val;
}

Value make_float(double v) {
    Value val;
    val.type = TYPE_FLOAT64;
    val.floatVal = v;
    return val;
}

Value make_bool(int v) {
    Value val;
    val.type = TYPE_BOOL;
    val.intVal = v ? 1 : 0;
    return val;
}

Value make_nil(void) {
    Value val;
    val.type = TYPE_INT;
    val.intVal = 0;
    return val;
}

Value make_string_n(const char* s, long n) {
    Value val;
    val.type = TYPE_STRING;
    char* buf = (char*)karkain_mem_alloc((size_t)n + 1);
    if (buf) {
        karkain_memcpy(buf, s, (size_t)n);
        buf[n] = '\0';
    }
    val.strVal = buf;
    return val;
}

Value make_string(const char* s) {
    return make_string_n(s, (long)karkain_strlen(s));
}

Value make_array(void) {
    Value val;
    val.type = TYPE_ARRAY;
    val.arrVal.items = NULL;
    val.arrVal.length = 0;
    return val;
}

ValueClass value_class(Value v) {
    switch (v.type) {
        case TYPE_INT:
        case TYPE_FLOAT64:
        case TYPE_BOOL:
            return VAL_IMMEDIATE;
        case TYPE_STRING:
        case TYPE_ARRAY:
            return VAL_HEAP;
    }
    return VAL_IMMEDIATE;
}

static const char* karkain_value_type_name(ValueType t) {
    switch (t) {
        case TYPE_INT: return "int";
        case TYPE_FLOAT64: return "float";
        case TYPE_STRING: return "string";
        case TYPE_ARRAY: return "array";
        case TYPE_BOOL: return "bool";
    }
    return "unknown";
}

const char* value_type_name(Value v) { return karkain_value_type_name(v.type); }

const char* value_type_tag(Value v) {
    switch (v.type) {
        case TYPE_INT: return "TYPE_INT";
        case TYPE_FLOAT64: return "TYPE_FLOAT64";
        case TYPE_STRING: return "TYPE_STRING";
        case TYPE_ARRAY: return "TYPE_ARRAY";
        case TYPE_BOOL: return "TYPE_BOOL";
    }
    return "TYPE_UNKNOWN";
}

long long value_as_int(Value v) {
    switch (v.type) {
        case TYPE_INT:
        case TYPE_BOOL:
            return v.intVal;
        case TYPE_FLOAT64:
            return (long long)v.floatVal;
        case TYPE_STRING:
            return karkain_strtoll(v.strVal ? v.strVal : "", NULL);
        default:
            return 0;
    }
}

double value_as_float(Value v) {
    switch (v.type) {
        case TYPE_FLOAT64:
            return v.floatVal;
        case TYPE_INT:
        case TYPE_BOOL:
            return (double)v.intVal;
        case TYPE_STRING:
            return (double)karkain_strtoll(v.strVal ? v.strVal : "", NULL);
        default:
            return 0.0;
    }
}

int values_equal(Value a, Value b) {
    if (a.type != b.type) return 0;
    switch (a.type) {
        case TYPE_INT:
        case TYPE_BOOL:
            return a.intVal == b.intVal;
        case TYPE_FLOAT64: {
            double diff = a.floatVal - b.floatVal;
            return (diff > -1e-9 && diff < 1e-9);
        }
        case TYPE_STRING:
            return a.strVal && b.strVal && karkain_strcmp(a.strVal, b.strVal) == 0;
        case TYPE_ARRAY: {
            if (a.arrVal.length != b.arrVal.length) return 0;
            for (int i = 0; i < a.arrVal.length; i++) {
                if (!values_equal(*a.arrVal.items[i], *b.arrVal.items[i])) return 0;
            }
            return 1;
        }
    }
    return 0;
}

int value_truthy(Value v) {
    switch (v.type) {
        case TYPE_INT:
        case TYPE_BOOL:
            return v.intVal != 0;
        case TYPE_FLOAT64:
            return v.floatVal != 0.0;
        case TYPE_STRING:
            return v.strVal && v.strVal[0] != '\0';
        case TYPE_ARRAY:
            return v.arrVal.length > 0;
    }
    return 0;
}