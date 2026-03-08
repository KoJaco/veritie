# Business Associate Potential with Google for pre-llm

https://cloud.google.com/security/compliance/hipaa

https://cloud.google.com/security/compliance/hipaa#covered-products

## HIPAA + LLMs (Google Gemini Context)

-   Google Cloud Platform (GCP) can sign a Business Associate Agreement (BAA).
-   Services explicitly listed as HIPAA-eligible under that BAA are the only ones where you can process PHI.

## IMPORTANT!

If you use Gemini through Vertex AI (with the right configuration + BAA in place), then you can legally send PHI to it.

If you're using Gemini via AI Studio or the public API directly, those endpoints are NOT the same as "Generative AI on Vertex AI" -> those would not fall under HIPAA coverage.

Nuance is:

1. Covered:

-   Gemini via Vertex AI -> HIPAA eligible with BAA.
-   Speech-to-text, Translation, Vision, Document AI -> also HIPAA eligible.

2. Not covered:

-   Gemini via consumer endpoints (AI Studio, public APIs outside Vertex).
-   Anything not explicitly listed.

## Obtaining a (Business Associate Addendum) BAA

https://support.google.com/cloud/answer/6329727
