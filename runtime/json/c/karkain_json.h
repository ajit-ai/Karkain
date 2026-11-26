#ifndef KARKAIN_JSON_H
#define KARKAIN_JSON_H

// v1.2.20: C23 JSON DOM runtime. The generated preamble defines Value,
// make_string/make_int/make_float/make_bool/make_map/make_array, map_set,
// array_push and array_get before this block is appended. No new link library.

Value karkain_json_parse(Value text);
Value karkain_json_type(Value node);
Value karkain_json_serialize(Value node);
Value karkain_json_get_path(Value root, Value path);
Value karkain_json_last_error(void);

#endif
