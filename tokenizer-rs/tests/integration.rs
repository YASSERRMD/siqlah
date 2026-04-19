use siqlah_tokenizer::merkle;

#[test]
fn boundary_root_determinism() {
    let offsets = vec![0u32, 5, 10, 15, 20];
    let r1 = merkle::boundary_root(&offsets);
    let r2 = merkle::boundary_root(&offsets);
    assert_eq!(r1, r2, "Merkle root should be deterministic");
}

#[test]
fn boundary_root_empty() {
    let root = merkle::boundary_root(&[]);
    assert_eq!(root.len(), 64, "should return 64 hex chars for zero hash");
}

#[test]
fn boundary_root_single() {
    let root = merkle::boundary_root(&[42]);
    assert_eq!(root.len(), 64);
}

#[test]
fn boundary_root_different_offsets() {
    let r1 = merkle::boundary_root(&[0, 1, 2]);
    let r2 = merkle::boundary_root(&[0, 1, 3]);
    assert_ne!(r1, r2, "different offsets should produce different roots");
}
