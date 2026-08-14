// Phase 17: Reflection runtime for Karkain
// Implements compile-time and runtime reflection primitives

#include <stdint.h>
#include <stdlib.h>
#include <string.h>
#include <stdio.h>

// Type kind enumeration
typedef enum {
    KIND_INVALID,
    KIND_INT,
    KIND_FLOAT64,
    KIND_STRING,
    KIND_BOOL,
    KIND_STRUCT,
    KIND_ARRAY,
    KIND_SLICE,
    KIND_MAP,
    KIND_PTR,
    KIND_FUNC,
    KIND_CHAN
} TypeKind;

// Field information structure
typedef struct {
    char* name;
    char* type_name;
    size_t offset;
    char** tags;
    size_t tag_count;
    int is_exported;
} FieldInfo;

// Type information structure
typedef struct {
    char* name;
    TypeKind kind;
    size_t size;
    size_t alignment;
    FieldInfo* fields;
    size_t field_count;
} TypeInfo;

// Reflection context
typedef struct {
    TypeInfo* types;
    size_t type_count;
} ReflectContext;

// Create reflection context
ReflectContext* reflect_context_create() {
    ReflectContext* ctx = (ReflectContext*)malloc(sizeof(ReflectContext));
    ctx->types = NULL;
    ctx->type_count = 0;
    return ctx;
}

// Destroy reflection context
void reflect_context_destroy(ReflectContext* ctx) {
    if (ctx == NULL) return;
    
    for (size_t i = 0; i < ctx->type_count; i++) {
        TypeInfo* type = &ctx->types[i];
        if (type->name) free(type->name);
        
        for (size_t j = 0; j < type->field_count; j++) {
            FieldInfo* field = &type->fields[j];
            if (field->name) free(field->name);
            if (field->type_name) free(field->type_name);
            
            for (size_t k = 0; k < field->tag_count; k++) {
                if (field->tags[k]) free(field->tags[k]);
            }
            if (field->tags) free(field->tags);
        }
        
        if (type->fields) free(type->fields);
    }
    
    if (ctx->types) free(ctx->types);
    free(ctx);
}

// Register a type in the reflection context
void reflect_register_type(ReflectContext* ctx, const char* name, TypeKind kind, size_t size, size_t alignment) {
    ctx->types = (TypeInfo*)realloc(ctx->types, (ctx->type_count + 1) * sizeof(TypeInfo));
    TypeInfo* type = &ctx->types[ctx->type_count];
    
    type->name = strdup(name);
    type->kind = kind;
    type->size = size;
    type->alignment = alignment;
    type->fields = NULL;
    type->field_count = 0;
    
    ctx->type_count++;
}

// Add a field to a struct type
void reflect_add_field(ReflectContext* ctx, const char* type_name, const char* field_name, 
                       const char* field_type, size_t offset, int is_exported) {
    TypeInfo* type = NULL;
    for (size_t i = 0; i < ctx->type_count; i++) {
        if (strcmp(ctx->types[i].name, type_name) == 0) {
            type = &ctx->types[i];
            break;
        }
    }
    
    if (type == NULL) return;
    
    type->fields = (FieldInfo*)realloc(type->fields, (type->field_count + 1) * sizeof(FieldInfo));
    FieldInfo* field = &type->fields[type->field_count];
    
    field->name = strdup(field_name);
    field->type_name = strdup(field_type);
    field->offset = offset;
    field->tags = NULL;
    field->tag_count = 0;
    field->is_exported = is_exported;
    
    type->field_count++;
}

// Add a tag to a field
void reflect_add_tag(ReflectContext* ctx, const char* type_name, const char* field_name, 
                    const char* tag_key, const char* tag_value) {
    TypeInfo* type = NULL;
    for (size_t i = 0; i < ctx->type_count; i++) {
        if (strcmp(ctx->types[i].name, type_name) == 0) {
            type = &ctx->types[i];
            break;
        }
    }
    
    if (type == NULL) return;
    
    FieldInfo* field = NULL;
    for (size_t i = 0; i < type->field_count; i++) {
        if (strcmp(type->fields[i].name, field_name) == 0) {
            field = &type->fields[i];
            break;
        }
    }
    
    if (field == NULL) return;
    
    // Store tag as "key:value" for simplicity
    char* tag_str = (char*)malloc(strlen(tag_key) + strlen(tag_value) + 2);
    sprintf(tag_str, "%s:%s", tag_key, tag_value);
    
    field->tags = (char**)realloc(field->tags, (field->tag_count + 1) * sizeof(char*));
    field->tags[field->tag_count] = tag_str;
    field->tag_count++;
}

// Get type information by name
TypeInfo* reflect_get_type(ReflectContext* ctx, const char* name) {
    for (size_t i = 0; i < ctx->type_count; i++) {
        if (strcmp(ctx->types[i].name, name) == 0) {
            return &ctx->types[i];
        }
    }
    return NULL;
}

// Get field information by name
FieldInfo* reflect_get_field(TypeInfo* type, const char* field_name) {
    for (size_t i = 0; i < type->field_count; i++) {
        if (strcmp(type->fields[i].name, field_name) == 0) {
            return &type->fields[i];
        }
    }
    return NULL;
}

// Get tag value from field
const char* reflect_get_tag(FieldInfo* field, const char* tag_key) {
    for (size_t i = 0; i < field->tag_count; i++) {
        if (strncmp(field->tags[i], tag_key, strlen(tag_key)) == 0) {
            // Return value after the colon
            const char* colon = strchr(field->tags[i], ':');
            if (colon) return colon + 1;
        }
    }
    return NULL;
}

// Get field count
size_t reflect_field_count(TypeInfo* type) {
    return type->field_count;
}

// Get field by index
FieldInfo* reflect_field_by_index(TypeInfo* type, size_t index) {
    if (index >= type->field_count) return NULL;
    return &type->fields[index];
}

// Type name to kind converter
TypeKind reflect_name_to_kind(const char* name) {
    if (strcmp(name, "int") == 0) return KIND_INT;
    if (strcmp(name, "float64") == 0) return KIND_FLOAT64;
    if (strcmp(name, "string") == 0) return KIND_STRING;
    if (strcmp(name, "bool") == 0) return KIND_BOOL;
    if (strncmp(name, "struct", 6) == 0) return KIND_STRUCT;
    if (strncmp(name, "[", 1) == 0) return KIND_ARRAY;
    if (strncmp(name, "[]", 2) == 0) return KIND_SLICE;
    if (strncmp(name, "map", 3) == 0) return KIND_MAP;
    if (strncmp(name, "*", 1) == 0) return KIND_PTR;
    if (strncmp(name, "func", 4) == 0) return KIND_FUNC;
    if (strncmp(name, "chan", 4) == 0) return KIND_CHAN;
    return KIND_INVALID;
}

// Get type size
size_t reflect_type_size(TypeInfo* type) {
    return type->size;
}

// Get type alignment
size_t reflect_type_alignment(TypeInfo* type) {
    return type->alignment;
}

// Check if type is valid
int reflect_type_valid(TypeInfo* type) {
    return type != NULL && type->kind != KIND_INVALID;
}

// Check if type is comparable
int reflect_type_comparable(TypeInfo* type) {
    switch (type->kind) {
        case KIND_FUNC:
        case KIND_MAP:
        case KIND_SLICE:
            return 0;
        default:
            return 1;
    }
}

// Deep equality check (simplified)
int reflect_deep_equal(const void* a, const void* b, TypeInfo* type) {
    // Simplified implementation - in real system would recursively check
    return memcmp(a, b, type->size) == 0;
}

// JSON serialization helper (simplified)
void reflect_to_json(void* value, TypeInfo* type, char* buffer, size_t buffer_size) {
    switch (type->kind) {
        case KIND_INT:
            snprintf(buffer, buffer_size, "%d", *(int*)value);
            break;
        case KIND_FLOAT64:
            snprintf(buffer, buffer_size, "%f", *(double*)value);
            break;
        case KIND_STRING:
            snprintf(buffer, buffer_size, "\"%s\"", *(char**)value);
            break;
        case KIND_BOOL:
            snprintf(buffer, buffer_size, "%s", *(int*)value ? "true" : "false");
            break;
        case KIND_STRUCT: {
            size_t offset = 0;
            offset += snprintf(buffer + offset, buffer_size - offset, "{");
            for (size_t i = 0; i < type->field_count; i++) {
                FieldInfo* field = &type->fields[i];
                void* field_ptr = (char*)value + field->offset;
                
                if (i > 0) {
                    offset += snprintf(buffer + offset, buffer_size - offset, ",");
                }
                
                const char* json_name = reflect_get_tag(field, "json");
                if (json_name == NULL) json_name = field->name;
                
                offset += snprintf(buffer + offset, buffer_size - offset, "\"%s\":", json_name);
                
                char field_buf[256];
                // Simplified - would need recursive call for full implementation
                offset += snprintf(buffer + offset, buffer_size - offset, "\"%s\"", field->type_name);
            }
            snprintf(buffer + offset, buffer_size - offset, "}");
            break;
        }
        default:
            snprintf(buffer, buffer_size, "null");
            break;
    }
}

// Initialize built-in types
void reflect_init_builtin_types(ReflectContext* ctx) {
    reflect_register_type(ctx, "int", KIND_INT, sizeof(int), _Alignof(int));
    reflect_register_type(ctx, "float64", KIND_FLOAT64, sizeof(double), _Alignof(double));
    reflect_register_type(ctx, "string", KIND_STRING, sizeof(char*), _Alignof(char*));
    reflect_register_type(ctx, "bool", KIND_BOOL, sizeof(int), _Alignof(int));
}

// Comptime evaluation context
typedef struct {
    void* symbols;
    size_t symbol_count;
} ComptimeContext;

// Create comptime context
ComptimeContext* comptime_context_create() {
    ComptimeContext* ctx = (ComptimeContext*)malloc(sizeof(ComptimeContext));
    ctx->symbols = NULL;
    ctx->symbol_count = 0;
    return ctx;
}

// Destroy comptime context
void comptime_context_destroy(ComptimeContext* ctx) {
    if (ctx == NULL) return;
    if (ctx->symbols) free(ctx->symbols);
    free(ctx);
}

// Evaluate comptime expression (simplified)
int comptime_eval_int(ComptimeContext* ctx, const char* expr) {
    // In a real implementation, this would parse and evaluate the expression
    // For now, return a placeholder value
    return 0;
}

// Comptime conditional
void* comptime_if(int condition, void* then_val, void* else_val) {
    return condition ? then_val : else_val;
}

// Generate derived method for JSON serialization
char* reflect_derive_json(TypeInfo* type) {
    // Calculate buffer size needed
    size_t buffer_size = 1024;
    char* buffer = (char*)malloc(buffer_size);
    size_t offset = 0;
    
    offset += snprintf(buffer + offset, buffer_size - offset, 
                      "func toJSON() string {\n"
                      "    var result string\n"
                      "    result += \"{\"\n");
    
    for (size_t i = 0; i < type->field_count; i++) {
        FieldInfo* field = &type->fields[i];
        if (i > 0) {
            offset += snprintf(buffer + offset, buffer_size - offset, "    result += \",\"\n");
        }
        
        const char* json_name = reflect_get_tag(field, "json");
        if (json_name == NULL) json_name = field->name;
        
        offset += snprintf(buffer + offset, buffer_size - offset, 
                          "    result += `\"%s:\" + toString(%s)\n", 
                          json_name, field->name);
    }
    
    offset += snprintf(buffer + offset, buffer_size - offset, 
                      "    result += \"}\"\n"
                      "    return result\n"
                      "}\n");
    
    return buffer;
}

// Generate derived method for String()
char* reflect_derive_stringer(TypeInfo* type) {
    size_t buffer_size = 1024;
    char* buffer = (char*)malloc(buffer_size);
    size_t offset = 0;
    
    offset += snprintf(buffer + offset, buffer_size - offset, 
                      "func String() string {\n"
                      "    return fmt.Sprintf(\"");
    
    for (size_t i = 0; i < type->field_count; i++) {
        if (i > 0) {
            offset += snprintf(buffer + offset, buffer_size - offset, " ");
        }
        offset += snprintf(buffer + offset, buffer_size - offset, 
                          "%s={%%v}", type->fields[i].name);
    }
    
    offset += snprintf(buffer + offset, buffer_size - offset, "\"");
    
    for (size_t i = 0; i < type->field_count; i++) {
        offset += snprintf(buffer + offset, buffer_size - offset, ", %s", type->fields[i].name);
    }
    
    offset += snprintf(buffer + offset, buffer_size - offset, ")\n}\n");
    
    return buffer;
}

// Generate derived method for equality
char* reflect_derive_eq(TypeInfo* type) {
    size_t buffer_size = 1024;
    char* buffer = (char*)malloc(buffer_size);
    size_t offset = 0;
    
    offset += snprintf(buffer + offset, buffer_size - offset, 
                      "func equals(other *%s) bool {\n"
                      "    if other == nil {\n"
                      "        return false\n"
                      "    }\n", type->name);
    
    for (size_t i = 0; i < type->field_count; i++) {
        offset += snprintf(buffer + offset, buffer_size - offset, 
                          "    if %s != other.%s {\n"
                          "        return false\n"
                          "    }\n", 
                          type->fields[i].name, type->fields[i].name);
    }
    
    offset += snprintf(buffer + offset, buffer_size - offset, 
                      "    return true\n"
                      "}\n");
    
    return buffer;
}