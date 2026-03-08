# ✅ PHI Pipeline Implementation – TODO List

#### 🟦 Domain Layer

-   [ ] Define `Redactor` interface in `domain/redaction.go`

```go
type Redactor interface {
    Redact(text string) (masked string, mapping map[string]string, err error)
}
```

---

#### 🟦 Infra Layer

**PII Redactor (existing)**

-   [ ] Confirm current `infra/pii_redactor` implements `domain.Redactor` interface

**PHI Redactor**

-   [ ] Create new `infra/phi_redactor` package
-   [ ] Export `Bio_ClinicalBERT` to ONNX

    ```
    optimum-cli export onnx \
      --model emilyalsentzer/Bio_ClinicalBERT \
      ./models/phi_clinicalbert
    ```

-   [ ] Load ONNX model in `New()` method
-   [ ] Implement `Redact()`:

    -   [ ] Run inference
    -   [ ] Identify PHI entity types (`PATIENT`, `DATE`, `CONDITION`, etc.)
    -   [ ] Build masked string
    -   [ ] Build `mapping` dictionary (placeholder → original)

---

#### 🟦 App Layer

-   [ ] Implement `RedactionService` that composes both PII + PHI redactors
-   [ ] Redact in this order: **PII → PHI**
-   [ ] Merge mappings

---

#### 🟦 Transport Layer

-   [ ] Call `RedactionService.Redact()` before invoking LLM
-   [ ] Store returned `mapping`
-   [ ] After LLM response, run `applyMapping()` to restore masked values for client-side response

---

#### 🟦 Main

-   [ ] Instantiate PII redactor
-   [ ] Instantiate PHI redactor
-   [ ] Inject into `RedactionService`
-   [ ] Pass `RedactionService` into transport dependencies

---

#### 🟦 Testing

| Test                             | Status |
| -------------------------------- | ------ |
| Unit – PII redaction             | \[ ]   |
| Unit – PHI redaction             | \[ ]   |
| Integration – combined redaction | \[ ]   |
| LLM smoke test (function call)   | \[ ]   |
| Mapping + restoration test       | \[ ]   |

---

#### 🟦 Optional Enhancements (Post-MVP)

-   [ ] Only run PHI redactor if `session.IsMedical == true`
-   [ ] Add `config.HIPAA_Mode` flag
-   [ ] Log % of tokens flagged as PHI for manual QA
-   [ ] Explore partial metadata injection in prompts (for better LLM context)
