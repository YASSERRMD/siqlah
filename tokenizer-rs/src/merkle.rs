use sha2::{Digest, Sha256};

const LEAF_PREFIX: u8 = 0x00;
const INTERNAL_PREFIX: u8 = 0x01;

pub fn hash_leaf(data: &[u8]) -> [u8; 32] {
    let mut h = Sha256::new();
    h.update([LEAF_PREFIX]);
    h.update(data);
    h.finalize().into()
}

pub fn hash_internal(left: &[u8; 32], right: &[u8; 32]) -> [u8; 32] {
    let mut h = Sha256::new();
    h.update([INTERNAL_PREFIX]);
    h.update(left);
    h.update(right);
    h.finalize().into()
}

fn split_point(n: usize) -> usize {
    let mut k = 1usize;
    while k < n {
        k <<= 1;
    }
    k >> 1
}

fn build_subtree(nodes: &[[u8; 32]]) -> [u8; 32] {
    if nodes.len() == 1 {
        return nodes[0];
    }
    let mid = split_point(nodes.len());
    let left = build_subtree(&nodes[..mid]);
    let right = build_subtree(&nodes[mid..]);
    hash_internal(&left, &right)
}

/// Computes the Merkle root over a slice of u32 token boundary offsets.
/// Returns the hex-encoded root string.
pub fn boundary_root(offsets: &[u32]) -> String {
    if offsets.is_empty() {
        return hex::encode([0u8; 32]);
    }
    let leaves: Vec<[u8; 32]> = offsets
        .iter()
        .map(|o| hash_leaf(&o.to_le_bytes()))
        .collect();
    hex::encode(build_subtree(&leaves))
}
