## MVP Tidy Up

This document describes the refactor to tidy up the MVPs comments and logging. It is to be initiated after the completion of the caching service

### Objectives

-   Switch from std lib logging to the logging package
-   Isolate startup, pipeline, and shutdown phases in logs with newlines
