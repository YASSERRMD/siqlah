use std::ffi::{CStr, CString};
use std::os::raw::c_char;

/// Tokenize text using a tokenizer specified as a JSON string.
/// Returns JSON: {"token_count":N,"boundary_root_hex":"...","tokenizer_hash":"..."}.
/// The caller must free the returned pointer with siqlah_free().
/// Returns null on any error.
#[no_mangle]
pub extern "C" fn siqlah_tokenize(
    text: *const c_char,
    tokenizer_json: *const c_char,
) -> *mut c_char {
    let text = match unsafe { cstr_to_str(text) } {
        Some(s) => s,
        None => return std::ptr::null_mut(),
    };
    let tokenizer_json = match unsafe { cstr_to_str(tokenizer_json) } {
        Some(s) => s,
        None => return std::ptr::null_mut(),
    };

    match crate::tokenize(text, tokenizer_json) {
        Ok(result) => match serde_json::to_string(&result) {
            Ok(json) => match CString::new(json) {
                Ok(cs) => cs.into_raw(),
                Err(_) => std::ptr::null_mut(),
            },
            Err(_) => std::ptr::null_mut(),
        },
        Err(_) => std::ptr::null_mut(),
    }
}

/// Free a string returned by siqlah_tokenize().
#[no_mangle]
pub extern "C" fn siqlah_free(ptr: *mut c_char) {
    if !ptr.is_null() {
        unsafe {
            drop(CString::from_raw(ptr));
        }
    }
}

unsafe fn cstr_to_str<'a>(ptr: *const c_char) -> Option<&'a str> {
    if ptr.is_null() {
        return None;
    }
    CStr::from_ptr(ptr).to_str().ok()
}
