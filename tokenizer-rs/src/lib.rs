pub mod ffi;
pub mod merkle;

use sha2::{Digest, Sha256};
use tokenizers::Tokenizer;

#[derive(Debug, serde::Serialize)]
pub struct TokenizeResult {
    pub token_count: usize,
    pub boundary_root_hex: String,
    pub tokenizer_hash: String,
}

/// Tokenize `text` using a tokenizer loaded from `tokenizer_json` (JSON string).
/// Returns token count, boundary Merkle root, and SHA256 of the tokenizer JSON.
pub fn tokenize(text: &str, tokenizer_json: &str) -> Result<TokenizeResult, String> {
    let tokenizer: Tokenizer = tokenizer_json
        .parse()
        .map_err(|e| format!("parse tokenizer JSON: {e}"))?;

    let encoding = tokenizer
        .encode(text, false)
        .map_err(|e| format!("encode: {e}"))?;

    let offsets: Vec<u32> = encoding
        .get_offsets()
        .iter()
        .map(|(start, _)| *start as u32)
        .collect();

    let boundary_root_hex = merkle::boundary_root(&offsets);

    let mut h = Sha256::new();
    h.update(tokenizer_json.as_bytes());
    let tokenizer_hash = hex::encode(h.finalize());

    Ok(TokenizeResult {
        token_count: encoding.len(),
        boundary_root_hex,
        tokenizer_hash,
    })
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn boundary_root_determinism() {
        let offsets = vec![0u32, 5, 10, 15, 20];
        let r1 = crate::merkle::boundary_root(&offsets);
        let r2 = crate::merkle::boundary_root(&offsets);
        assert_eq!(r1, r2);
    }

    #[test]
    fn boundary_root_empty() {
        let root = crate::merkle::boundary_root(&[]);
        assert_eq!(root.len(), 64);
    }
}
