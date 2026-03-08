# Proposed ENV for redaction-based stuff

determines HIPAA eligibility path

-   LLM_BACKEND=vertex|aistudio

only true if you have signed BAA + vertex

-   HIPAA_BAA=true|false

vertex region for routing

-   REGION=us-central1

gcp project for vertex

-   PROJECT_ID=...

auto = (LLM_BACKEND!=vertex || !HIPAA_BAA)

-   REDACT_PHI_PRE_LLM=auto

PCI never goes to LLM

-   REDACT_PCI_PRE_LLM=true

user/org level toggle

-   REDACT_PII_PRE_LLM=opt-in

org-level; if true, encrypt at rest; short TTL

-   STORE_RAW_TRANSCRIPTS=false
