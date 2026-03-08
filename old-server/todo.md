# Todos For Thursday & Friday

1. Implement PCI

2. Implement PII, golang only rules based (default off)

3. Add in testing ability for users (mark session as testing via config, discount rates, etc)

4. Cursor fix frontend / server bugs

-   Structured output and Function schema checksums not seeming to work, need to also attach parsing guide in function parsing schemas... small ui bugs... maybe if the name and description is the same just generate the checksum on the function declaration/structured output schema

5. Get batch working (much cheaper option, more robust)

-   Add in Whisper as an option for Batch or just use deepgram endpoint.

6. Add the ability to adjust update frequency and add that affect into pipeline for hitting LLM (cost reduction)

-   Should also add ability to do an end-of-session pass. Don't use LLM in real-time, wait to audio is finished recorded then do a pass... should be simple to add to the pipelines.

7. Update UI

-   Get things looking how I want them to look
-   Proper features
-   Relabel docs in dashboard to be 'integration'. Make it integration specific... link out to main docs (public route)

8. Deployment

-   Deploy server to fly with 2x sidecars and extra models in storage '/data/models/
-   Purchase domain name
-   Deploy frontend (staging for now, no index)
