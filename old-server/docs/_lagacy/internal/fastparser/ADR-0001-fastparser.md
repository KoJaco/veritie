| Decision                      | Value                                                                                                             |
| ----------------------------- | ----------------------------------------------------------------------------------------------------------------- |
| **Fast‑parser**               | Embedding‑based retrieval (BGE‑small) + optional zero‑shot intent classifier (same embeddings)                    |
| **No rule grammars**          | ✅                                                                                                                |
| **No self‑hosted large LLM**  | ✅                                                                                                                |
| **Side‑car vs library**       | **Start as in‑repo Go package**. Break out into a Fly side‑car only if latency or process‑size becomes a concern. |
| **Function object lifecycle** | `status: "draft" → "model" → "retracted"` with `confidence` float.                                                |
