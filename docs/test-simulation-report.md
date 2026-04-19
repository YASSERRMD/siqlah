# Siqlah v0.2 — Full Cycle Simulation Report

> Generated: 2026-04-19T16:09:21Z
> Test: `TestFullCycleSimulation` in `test/simulation_test.go`

This document records end-to-end evidence for the complete siqlah audit trail cycle:
ingest → checkpoint → inclusion proof → C2SP witness cosigning → in-toto attestation → consistency proof.


---

## Operator Key

| Field | Value |
|---|---|
| Algorithm | Ed25519 |
| Public Key (hex) | `4c5d7588a72c939a2a255a1bb5082fc7c2855771aaaeaa6c536e419a7ab309a9` |


---

## Phase 1 — Receipt Inventory (50 logs)

All 50 receipts ingested via `POST /v1/receipts`. Each is signed by the operator's Ed25519 key at ingest time.

| # | Provider | Model | Tenant | Use Case | In Tokens | Out Tokens | Receipt ID |
|---|---|---|---|---|---|---|---|
| 01 | openai | gpt-4o | fintech-corp | fraud-detection | 342 | 128 | `016a1252-ce31-4a96-9c31-e0631899515b` |
| 02 | openai | gpt-4o | fintech-corp | risk-scoring | 512 | 256 | `64f05714-db46-4213-8878-d13b723b5160` |
| 03 | openai | gpt-4o | healthcare-sys | clinical-summarization | 1024 | 512 | `a46e870f-3e92-4293-b898-022221fdfe8c` |
| 04 | openai | gpt-4o | healthcare-sys | icd-coding | 780 | 190 | `fc22e24a-b919-43ba-bb8f-d6052f1ae043` |
| 05 | openai | gpt-4o | legal-ai | contract-review | 2048 | 768 | `1bbbbeca-6e68-46e3-9b6e-e45ec37afb5c` |
| 06 | openai | gpt-4-turbo | legal-ai | clause-extraction | 1500 | 400 | `8e657f46-2254-4595-a715-d69ade7a9b4a` |
| 07 | openai | gpt-4-turbo | devtools-co | code-completion | 300 | 150 | `215581d8-8c59-4d8a-8ac2-cb9ad27d4549` |
| 08 | openai | gpt-4-turbo | devtools-co | bug-fix | 600 | 200 | `e76c5ec6-8438-4c33-b9ae-85749df4fb16` |
| 09 | openai | gpt-4-turbo | gaming-studio | npc-dialogue | 250 | 300 | `47419639-5f97-417b-a524-e9135ef7574d` |
| 10 | openai | gpt-4-turbo | gaming-studio | story-generation | 800 | 600 | `de2f2b1c-7151-4e26-bfde-c6c872b5cd18` |
| 11 | openai | gpt-3.5-turbo | media-co | headline-generation | 200 | 60 | `ad3afb4a-3f90-4c98-9800-51d0cec37a05` |
| 12 | openai | gpt-3.5-turbo | media-co | article-summary | 1200 | 300 | `b2297465-0c7e-4ad7-8ce3-9e11500e16f6` |
| 13 | openai | gpt-3.5-turbo | fintech-corp | alert-triage | 180 | 80 | `3fa5ffd7-efd8-4f01-8a9f-1910f1dd59be` |
| 14 | openai | gpt-3.5-turbo | devtools-co | doc-generation | 450 | 220 | `b66af887-08c1-4d24-a5c8-bdfaf50ec4a2` |
| 15 | openai | gpt-3.5-turbo | healthcare-sys | patient-faq | 320 | 140 | `3e491d7b-38b0-48e8-a03f-31f327715a49` |
| 16 | openai | gpt-4o-mini | gaming-studio | item-description | 150 | 80 | `ea5ddc8c-138c-4665-b3c9-99fbf33aff75` |
| 17 | openai | gpt-4o-mini | media-co | tag-generation | 100 | 40 | `861549fc-1190-4d21-baee-86eafba56d74` |
| 18 | openai | gpt-4o-mini | legal-ai | quick-summary | 600 | 120 | `a1742500-a9e0-4de7-b754-49fea45fc25d` |
| 19 | openai | gpt-4o-mini | devtools-co | test-generation | 400 | 200 | `69230d19-a7b9-4c3c-b346-21ed0a4976ea` |
| 20 | openai | gpt-4o-mini | fintech-corp | sentiment-analysis | 250 | 50 | `7e313791-c099-4ffd-86b3-4a3ae41243bf` |
| 21 | openai | o1-preview | devtools-co | architecture-review | 1800 | 900 | `14fd0323-c253-47c7-a8f2-d8d10e2db0f8` |
| 22 | openai | o1-preview | fintech-corp | model-validation | 2200 | 1100 | `de26fa91-0291-4c22-afac-56d21c304b97` |
| 23 | openai | o1-preview | healthcare-sys | diagnosis-assist | 1600 | 800 | `68d1e3f9-f592-4a3a-8372-d5d02a5f4dde` |
| 24 | openai | o1-preview | legal-ai | case-analysis | 3000 | 1500 | `8384163f-4cf0-4e9c-b5c6-28c74f2b26fb` |
| 25 | openai | o1-preview | gaming-studio | game-design | 900 | 450 | `10ddd71f-6b3c-4132-b0c0-9943724b47eb` |
| 26 | anthropic | claude-3-opus-20240229 | fintech-corp | compliance-check | 2400 | 800 | `a424cb33-70d8-42df-9427-c75285e8e206` |
| 27 | anthropic | claude-3-opus-20240229 | healthcare-sys | research-synthesis | 3200 | 1200 | `d26b532c-c5db-4ff4-a849-289b29d14115` |
| 28 | anthropic | claude-3-opus-20240229 | legal-ai | due-diligence | 4096 | 1600 | `b7093f0d-ed00-4753-9e05-d4f05afce575` |
| 29 | anthropic | claude-3-opus-20240229 | media-co | editorial-review | 1800 | 700 | `340bcb53-4f24-4374-8661-797a58bc6b3a` |
| 30 | anthropic | claude-3-opus-20240229 | devtools-co | refactoring | 2000 | 900 | `ae352c69-0e09-40c2-984d-396c89c15da8` |
| 31 | anthropic | claude-3-5-sonnet-20241022 | gaming-studio | world-building | 1200 | 500 | `bb4899f0-4b9a-4c3a-88fa-42ca77314b0f` |
| 32 | anthropic | claude-3-5-sonnet-20241022 | fintech-corp | report-drafting | 1400 | 600 | `1a5ef935-8c11-4917-ab71-6f5fe0a1b244` |
| 33 | anthropic | claude-3-5-sonnet-20241022 | healthcare-sys | medication-info | 600 | 280 | `9f205273-5c1f-4d15-9b61-c5d9ad75e972` |
| 34 | anthropic | claude-3-5-sonnet-20241022 | legal-ai | argument-outline | 1000 | 420 | `dc057f72-d030-480f-9535-ff0a5df96765` |
| 35 | anthropic | claude-3-5-sonnet-20241022 | devtools-co | api-design | 750 | 350 | `b83d639c-a124-4954-b018-64d402477bac` |
| 36 | anthropic | claude-3-sonnet-20240229 | media-co | translation | 800 | 820 | `a4f0d7c7-801e-473b-8dcc-327ebca0050b` |
| 37 | anthropic | claude-3-sonnet-20240229 | fintech-corp | data-extraction | 950 | 400 | `bd26ba2e-6650-4fa2-86cc-284bda4ff4de` |
| 38 | anthropic | claude-3-sonnet-20240229 | gaming-studio | quest-design | 700 | 350 | `a354706b-5600-418f-8f05-23068317be28` |
| 39 | anthropic | claude-3-haiku-20240307 | devtools-co | lint-explanation | 300 | 120 | `10723145-734e-4dcc-ab1e-5f3b9662b6c4` |
| 40 | anthropic | claude-3-haiku-20240307 | media-co | caption-generation | 180 | 60 | `b9a2bdfb-8094-4d00-96e7-8412afcf783f` |
| 41 | generic | llama-3-8b | devtools-co | code-review | 400 | 180 | `5efeeb92-fca0-4aed-b93b-fd7364d3550d` |
| 42 | generic | llama-3-8b | gaming-studio | flavor-text | 200 | 150 | `d245d5b0-553c-4716-82d4-20a5e0c8662c` |
| 43 | generic | llama-3-70b | media-co | long-form-content | 2000 | 1200 | `39631b20-0b06-4b32-9f8d-d930e89d41c9` |
| 44 | generic | mistral-7b-instruct | healthcare-sys | note-summarization | 500 | 200 | `da97abe0-2620-4a7a-8d23-ea8b45fb5665` |
| 45 | generic | mistral-7b-instruct | fintech-corp | email-triage | 350 | 100 | `482c066c-e933-4b92-bb1a-a03e26ef269c` |
| 46 | anthropic | claude-3-5-sonnet-20241022 | healthcare-sys | trial-matching | 1100 | 480 | `6e5ba84b-1b33-4c84-83c3-1675cd983ba4` |
| 47 | openai | gpt-4o | media-co | interview-transcription | 4200 | 900 | `6eaac7e0-3c8e-48c7-a67d-c49567473c9b` |
| 48 | openai | gpt-4o | devtools-co | security-audit | 2600 | 700 | `884ec49d-8478-476b-a653-1103352e9702` |
| 49 | generic | llama-3-70b | legal-ai | contract-summarization | 3000 | 800 | `ba36fab4-8d3a-4cee-a58d-704e71ef7ce5` |
| 50 | anthropic | claude-3-opus-20240229 | gaming-studio | lore-generation | 1600 | 750 | `0b01ca76-2f64-44a3-94e3-447d26d4a100` |

> **Energy**: 22/50 receipts carry energy estimates. Total estimated energy: 97.1420 J


---

## Phase 2 — Checkpoint 1

Built immediately after ingesting all 50 receipts.

| Field | Value |
|---|---|
| Checkpoint ID | `1` |
| Batch Start (row) | `1` |
| Batch End (row) | `50` |
| Tree Size | `50` |
| Merkle Root (hex) | `120497228fe58aa01758b2c1074931cc9b0b93499e886125cdda4f5154a6db09` |
| Previous Root | `` |
| Issued At | `2026-04-19T16:09:21Z` |
| Operator Sig | `b4621f6883d5273f3e798de636eedb24...` |
| Rekor Log Index | `0` |


---

## Phase 3 — C2SP Witness Cosigning

Two independent witnesses fetched the C2SP signed note from `GET /v1/witness/checkpoint`,
verified the operator signature, appended their own Ed25519 cosignature,
and submitted via `POST /v1/witness/cosign`.

| Witness | Result | Notes |
|---|---|---|
| `simulation-witness-1` | ✓ Success | Cosignature accepted |
| `simulation-witness-2` | ✓ Success | Cosignature accepted |

### Cosigned Note (merged, 3 signatures)

```
siqlah.dev/log
50
EgSXIo/liqAXWLLBB0kxzJsLk0meiGElzdpPUVSm2wk=
— siqlah.dev/log S0OefW0yT/F9wFMtbV2PMqJKFKLrH8lK4On0i2Ed1bX6x7YGcTFRnpdkRkQQ8xGCf3ql5LRlJjDve//N/eHPs+d0iQ4=
— simulation-witness-1 ZZIbHLGZik0I6YdoMgvYvwNkFLNkIdROOSRjryv6BvyhHikHlJTUBEp1yVLlP1eBXf87QLkZL7ReNk7SpoasXJJ9XQ4=
— simulation-witness-2 ify6gcz9ITWWUT0gkwrJF/lDREQ6O1nCjWsgPpXtvEYaKthK0zZWejDxUAqtPtZMYSrxyMCpIjua5TOA+dBzUWLx3Ag=
```


---

## Phase 4 & 5 — Additional Receipts and Checkpoint 2

10 additional receipts were ingested to demonstrate consistency proofs.

| # | Provider | Model | Tenant | Receipt ID |
|---|---|---|---|---|
| 1 | openai | gpt-4o | fintech-corp | `58b1a7f3-bc1f-4f98-b897-f43f112420d8` |
| 2 | anthropic | claude-3-opus-20240229 | legal-ai | `231f1ec9-ea73-42d4-9332-3af206b66d0d` |
| 3 | generic | llama-3-8b | devtools-co | `181e3db0-d2c5-4787-8ca0-0913606d5877` |
| 4 | openai | gpt-4o-mini | healthcare-sys | `752e8914-dfe8-45e6-9886-fa5ccea29542` |
| 5 | anthropic | claude-3-haiku-20240307 | gaming-studio | `29000349-ccf5-4c0c-8e3b-848f68840a65` |
| 6 | openai | gpt-4-turbo | media-co | `753f1a76-0b47-4e27-9404-c66ad38ba1d1` |
| 7 | generic | mistral-7b-instruct | fintech-corp | `1bd1ac14-52e7-449e-aa41-dbf25480a6e3` |
| 8 | openai | o1-preview | legal-ai | `1eedd5d1-bcd0-4fe9-b613-f95708a01bf2` |
| 9 | anthropic | claude-3-5-sonnet-20241022 | devtools-co | `b12f10aa-e8a6-4bcb-895f-b1c974f35afd` |
| 10 | openai | gpt-4o | gaming-studio | `ada83117-9541-46e8-ac38-7c7a1cef1611` |

**Checkpoint 2:**

| Field | Value |
|---|---|
| Checkpoint ID | `2` |
| Tree Size | `10` |
| Merkle Root (hex) | `33761dc5b84df70b62e58626a3a89ed9bff7fdac445a78850fc770e1e09dba51` |
| Previous Root | `120497228fe58aa01758b2c1074931cc9b0b93499e886125cdda4f5154a6db09` |


---

## Phase 6 — Inclusion Proofs (all 50 receipts)

Every receipt's inclusion proof was fetched from `GET /v1/receipts/{id}/proof`
and **locally verified** client-side using `merkle.VerifyInclusion`.

**Summary: 50/50 proofs passed local verification** (0 failures expected)

| # | Receipt ID | Leaf Index | Tree Size | Proof Len | Local Verify |
|---|---|---|---|---|---|
| 01 | `016a1252-ce31-4a96-9c31-e0631899515b` | 0 | 50 | 6 | ✓ PASS |
| 02 | `64f05714-db46-4213-8878-d13b723b5160` | 1 | 50 | 6 | ✓ PASS |
| 03 | `a46e870f-3e92-4293-b898-022221fdfe8c` | 2 | 50 | 6 | ✓ PASS |
| 04 | `fc22e24a-b919-43ba-bb8f-d6052f1ae043` | 3 | 50 | 6 | ✓ PASS |
| 05 | `1bbbbeca-6e68-46e3-9b6e-e45ec37afb5c` | 4 | 50 | 6 | ✓ PASS |
| 06 | `8e657f46-2254-4595-a715-d69ade7a9b4a` | 5 | 50 | 6 | ✓ PASS |
| 07 | `215581d8-8c59-4d8a-8ac2-cb9ad27d4549` | 6 | 50 | 6 | ✓ PASS |
| 08 | `e76c5ec6-8438-4c33-b9ae-85749df4fb16` | 7 | 50 | 6 | ✓ PASS |
| 09 | `47419639-5f97-417b-a524-e9135ef7574d` | 8 | 50 | 6 | ✓ PASS |
| 10 | `de2f2b1c-7151-4e26-bfde-c6c872b5cd18` | 9 | 50 | 6 | ✓ PASS |
| 11 | `ad3afb4a-3f90-4c98-9800-51d0cec37a05` | 10 | 50 | 6 | ✓ PASS |
| 12 | `b2297465-0c7e-4ad7-8ce3-9e11500e16f6` | 11 | 50 | 6 | ✓ PASS |
| 13 | `3fa5ffd7-efd8-4f01-8a9f-1910f1dd59be` | 12 | 50 | 6 | ✓ PASS |
| 14 | `b66af887-08c1-4d24-a5c8-bdfaf50ec4a2` | 13 | 50 | 6 | ✓ PASS |
| 15 | `3e491d7b-38b0-48e8-a03f-31f327715a49` | 14 | 50 | 6 | ✓ PASS |
| 16 | `ea5ddc8c-138c-4665-b3c9-99fbf33aff75` | 15 | 50 | 6 | ✓ PASS |
| 17 | `861549fc-1190-4d21-baee-86eafba56d74` | 16 | 50 | 6 | ✓ PASS |
| 18 | `a1742500-a9e0-4de7-b754-49fea45fc25d` | 17 | 50 | 6 | ✓ PASS |
| 19 | `69230d19-a7b9-4c3c-b346-21ed0a4976ea` | 18 | 50 | 6 | ✓ PASS |
| 20 | `7e313791-c099-4ffd-86b3-4a3ae41243bf` | 19 | 50 | 6 | ✓ PASS |
| 21 | `14fd0323-c253-47c7-a8f2-d8d10e2db0f8` | 20 | 50 | 6 | ✓ PASS |
| 22 | `de26fa91-0291-4c22-afac-56d21c304b97` | 21 | 50 | 6 | ✓ PASS |
| 23 | `68d1e3f9-f592-4a3a-8372-d5d02a5f4dde` | 22 | 50 | 6 | ✓ PASS |
| 24 | `8384163f-4cf0-4e9c-b5c6-28c74f2b26fb` | 23 | 50 | 6 | ✓ PASS |
| 25 | `10ddd71f-6b3c-4132-b0c0-9943724b47eb` | 24 | 50 | 6 | ✓ PASS |
| 26 | `a424cb33-70d8-42df-9427-c75285e8e206` | 25 | 50 | 6 | ✓ PASS |
| 27 | `d26b532c-c5db-4ff4-a849-289b29d14115` | 26 | 50 | 6 | ✓ PASS |
| 28 | `b7093f0d-ed00-4753-9e05-d4f05afce575` | 27 | 50 | 6 | ✓ PASS |
| 29 | `340bcb53-4f24-4374-8661-797a58bc6b3a` | 28 | 50 | 6 | ✓ PASS |
| 30 | `ae352c69-0e09-40c2-984d-396c89c15da8` | 29 | 50 | 6 | ✓ PASS |
| 31 | `bb4899f0-4b9a-4c3a-88fa-42ca77314b0f` | 30 | 50 | 6 | ✓ PASS |
| 32 | `1a5ef935-8c11-4917-ab71-6f5fe0a1b244` | 31 | 50 | 6 | ✓ PASS |
| 33 | `9f205273-5c1f-4d15-9b61-c5d9ad75e972` | 32 | 50 | 6 | ✓ PASS |
| 34 | `dc057f72-d030-480f-9535-ff0a5df96765` | 33 | 50 | 6 | ✓ PASS |
| 35 | `b83d639c-a124-4954-b018-64d402477bac` | 34 | 50 | 6 | ✓ PASS |
| 36 | `a4f0d7c7-801e-473b-8dcc-327ebca0050b` | 35 | 50 | 6 | ✓ PASS |
| 37 | `bd26ba2e-6650-4fa2-86cc-284bda4ff4de` | 36 | 50 | 6 | ✓ PASS |
| 38 | `a354706b-5600-418f-8f05-23068317be28` | 37 | 50 | 6 | ✓ PASS |
| 39 | `10723145-734e-4dcc-ab1e-5f3b9662b6c4` | 38 | 50 | 6 | ✓ PASS |
| 40 | `b9a2bdfb-8094-4d00-96e7-8412afcf783f` | 39 | 50 | 6 | ✓ PASS |
| 41 | `5efeeb92-fca0-4aed-b93b-fd7364d3550d` | 40 | 50 | 6 | ✓ PASS |
| 42 | `d245d5b0-553c-4716-82d4-20a5e0c8662c` | 41 | 50 | 6 | ✓ PASS |
| 43 | `39631b20-0b06-4b32-9f8d-d930e89d41c9` | 42 | 50 | 6 | ✓ PASS |
| 44 | `da97abe0-2620-4a7a-8d23-ea8b45fb5665` | 43 | 50 | 6 | ✓ PASS |
| 45 | `482c066c-e933-4b92-bb1a-a03e26ef269c` | 44 | 50 | 6 | ✓ PASS |
| 46 | `6e5ba84b-1b33-4c84-83c3-1675cd983ba4` | 45 | 50 | 6 | ✓ PASS |
| 47 | `6eaac7e0-3c8e-48c7-a67d-c49567473c9b` | 46 | 50 | 6 | ✓ PASS |
| 48 | `884ec49d-8478-476b-a653-1103352e9702` | 47 | 50 | 6 | ✓ PASS |
| 49 | `ba36fab4-8d3a-4cee-a58d-704e71ef7ce5` | 48 | 50 | 3 | ✓ PASS |
| 50 | `0b01ca76-2f64-44a3-94e3-447d26d4a100` | 49 | 50 | 3 | ✓ PASS |

> All proofs use the same Merkle root `120497228fe58aa01758b2c1...`


---

## Phase 7 — Operator Signature Verification

The checkpoint's `operator_sig_hex` covers the canonical `SignedPayload`
(batch bounds, tree size, root, previous root, timestamp).

| Checkpoint | Operator Valid |
|---|---|
| CP1 | ✓ VALID (verified against operator public key `4c5d7588a72c939a...`) |


---

## Phase 9 — Consistency Proof (CP1 → CP2)

Proves that Checkpoint 2 (tree of size 10) is a valid extension of Checkpoint 1 (tree of size 50).

| Field | Value |
|---|---|
| Old Checkpoint ID | `1` |
| New Checkpoint ID | `2` |
| Old Size | `50` |
| New Size | `60` |
| Old Root | `120497228fe58aa01758b2c1074931cc9b0b93499e886125cdda4f5154a6db09` |
| New Root | `cf4037f817d4a6c78bdcfb1563639be11bc1ae9c78b4c9fc46899eecc1c51a3c` |
| Proof Elements | `6` |

**Proof hashes:**

- `[0]` `79a5cc91581b9e1ee3d383e3cb9a744d3e607ffab96ab1a38a1a6c590fb0a47f`
- `[1]` `04cc62dc8f996d20728f5fb7c37989648273940d4065a6b1f63d70e6d6c0f623`
- `[2]` `4fc69e230e73225bc247cdd856542055fd373d6755669d381b4e8519b8ba1b98`
- `[3]` `56503de0d06a47b51107e6997ea12862286a65e19b15ffc629238ce173a961dd`
- `[4]` `fc83066a06c8850a6b0e379745050707946a5bed8f23d0480a9e3ff3e5253e97`
- `[5]` `a42fcdfdaee230a36025f875c7d6cf44d5e90e29ead6965cce692f7e653e0821`


---

## Phase 10 — In-Toto Attestations (10 sample receipts)

Attestations fetched from `GET /v1/receipts/{id}/attestation`.
Each attestation is an in-toto v1 Statement with predicate type `https://siqlah.dev/receipt/v1`.

| Sample# | Receipt ID | _type | predicateType | Subjects | Valid |
|---|---|---|---|---|---|
| 01 | `016a1252` | `https://in-toto.io/Statement/v1` | `https://siqlah.dev/receipt/v1` | 1 | ✓ |
| 06 | `8e657f46` | `https://in-toto.io/Statement/v1` | `https://siqlah.dev/receipt/v1` | 1 | ✓ |
| 11 | `ad3afb4a` | `https://in-toto.io/Statement/v1` | `https://siqlah.dev/receipt/v1` | 1 | ✓ |
| 16 | `ea5ddc8c` | `https://in-toto.io/Statement/v1` | `https://siqlah.dev/receipt/v1` | 1 | ✓ |
| 21 | `14fd0323` | `https://in-toto.io/Statement/v1` | `https://siqlah.dev/receipt/v1` | 1 | ✓ |
| 26 | `a424cb33` | `https://in-toto.io/Statement/v1` | `https://siqlah.dev/receipt/v1` | 1 | ✓ |
| 31 | `bb4899f0` | `https://in-toto.io/Statement/v1` | `https://siqlah.dev/receipt/v1` | 1 | ✓ |
| 36 | `a4f0d7c7` | `https://in-toto.io/Statement/v1` | `https://siqlah.dev/receipt/v1` | 1 | ✓ |
| 41 | `5efeeb92` | `https://in-toto.io/Statement/v1` | `https://siqlah.dev/receipt/v1` | 1 | ✓ |
| 50 | `0b01ca76` | `https://in-toto.io/Statement/v1` | `https://siqlah.dev/receipt/v1` | 1 | ✓ |

Each attestation's `subject` array contains two entries:
- `request:<receipt-id>` with digest `sha256:<request_hash>`
- `response:<receipt-id>` with digest `sha256:<response_hash>`


---

## Phase 11 — System Stats

| Metric | Value |
|---|---|
| Total Receipts | `60` |
| Total Checkpoints | `2` |
| Pending Batch | `0` |
| Witness Signatures | `0` |


---

## Summary

| Step | Result |
|---|---|
| 50 receipts ingested | ✓ All 201 Created |
| Checkpoint 1 built (50 receipts) | ✓ id=1 size=50 |
| Witness 1 C2SP cosign | ✓ Success |
| Witness 2 C2SP cosign | ✓ Success |
| Cosigned note signatures | ✓ 3 signature(s) |
| Checkpoint 2 built (10 more receipts) | ✓ id=2 size=10 |
| Inclusion proofs (50) | ✓ 50/50 locally verified |
| Operator signature verification | ✓ VALID |
| Consistency proof (CP1→CP2) | ✓ 6 elements |
| In-toto attestations (10 samples) | ✓ All valid |

**All 50 receipts are fully auditable:** inclusion-proved, operator-signed,
cosigned by 2 independent witnesses, and exposable as in-toto SLSA attestations.

