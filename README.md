# Buildbarn Event Service

Buildbarn Event Service is an implementation of
[Bazel's Build Event Service](https://docs.bazel.build/versions/master/build-event-protocol.html#the-build-event-service).
In short, this service can be used to log and store all sorts of
information regarding invocations of Bazel. Information includes the
list of targets analyzed, whether they could be built, output files that
were generated and test results.

This service heavily builds on top of other Buildbarn components.
Instead of having its own storage layer, it can store its results into
[the Buildbarn storage daemon](https://github.com/buildbarn/bb-storage).
Build event streams are stored in the Content Addressable Storage (CAS),
while entries that permit lookups of these streams by Bazel invocation
ID (a per-invocation UUID) are stored in the Action Cache (AC).

[Buildbarn Browser](https://github.com/buildbarn/bb-browser) has
integrated support for displaying build event streams stored in the
CAS/AC by visiting `/build_events/<instance>/<uuid>`. For builds that
also used remote execution, Buildbarn Browser is capable of linking
directly to output files of build actions, making it a valuable tool for
sharing build results with colleagues. By invoking Bazel with
`--bes_results_url=...`, it may automatically print URLs pointing to
Buildbarn Browser at the start of every invocation.

Prebuilt container images of Buildbarn Event Service may be found on
[Docker Hub](https://hub.docker.com/r/buildbarn/bb-event-service).
Examples of how the Buildbarn Event Service may be deployed and used can
be found in [the Buildbarn deployments repository](https://github.com/buildbarn/bb-deployments).
