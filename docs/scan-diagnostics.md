# Scan and import diagnostics

The Import page shows the latest local `scan-diagnostic.jsonl` log and provides
a Copy button. A new guided scan replaces the previous log. The log is not
uploaded automatically. The detailed list scrolls independently of the page.
Decoder stages are written as they occur so an interrupted helper leaves its
last reached stage in the log. The Import page reloads the log after the scan.

## Stages

- Capture and conversion: Windows capture status, packet reading and duplicate
  packet removal.
- Capture coverage: observed source-packet duration compared with the requested
  guided window. A short interval warns about quiet traffic or overwritten
  events; it does not prove either explanation.
- Packet selection: source, selected and excluded counts. The source is streamed
  and only supported transport packets are retained for decoding.
- UDP reconstruction: Unreal packet, bunch, fragmented bunch and completed
  message counts. A fragmented bunch count is not a lost-message count.
- Decoder inputs: raw payload, reconstructed inventory stream and completed
  message counts. Domain parsers continue to use their existing input sources.
- Equipment, characters, Arcs and resources: recognized-name candidates,
  accepted sightings, unique records retained and rejections by parser phase.
  Character records can contain an additional opaque word among their panel
  fields. The scanner checks the following skill array before selecting that layout.
- Character observations: counts without associated panels, observed bonds or
  skill levels. Missing optional observations are not inferred.
- Equipment associations: unresolved character/Arc references and equipped
  modules without a recovered placement.
- Module placement evidence: validated cell groups in eligible and excluded
  sources, rejected cell/group candidates and modules with no recognized
  placement candidate.
- Furniture: channel 4 availability, literal property name detection, envelope
  checks, LZ4 decoding and decompressed record validation.
- Validation and export: checks the required domains for guided scans and
  validates record relationships before writing the export. Equipment with an
  owner missing from the decoded roster is retained as unassigned, without a
  grid placement. Validation is marked `partial` with the number of affected
  records, and the export can complete.
- Import: catalog loading, typed JSON decoding for each domain, export version,
  character identities, equipment validation, Arc links, furniture validation
  and atomic account snapshot publication.

`complete` means the checks for that stage passed. `partial` indicates missing
data or an association the scanner can omit while producing a valid import. A
failed import retains its diagnostic stages without publishing an account snapshot.

## Interpreting record counters

Guided capture records complete packets at NIC components instead of every
network-processing component. The circular ETL journal has an explicit 1,024 MB
limit. This reduces duplicate observations and the chance that unrelated traffic
overwrites login packets. Existing system-wide Pktmon filters are left untouched.
Circular capture can still overwrite older events under sufficiently heavy load.
Microsoft documents these capture settings in
[Pktmon start](https://learn.microsoft.com/en-us/windows-server/administration/windows-commands/pktmon-start).

Packet selection reads PCAPNG incrementally with a 1 GiB source limit, a 16 MiB
per-block limit and a retained set limited to 256 MiB of packet bytes and one
million packets. It retains IPv4 Ethernet UDP packets and TCP port 30031, matching
the scanner's current transport consumers. Unrelated TCP bytes are never retained
in the packet set. Unsupported network formats are not inferred.

The coverage interval uses all source packets, including excluded TCP traffic,
so filtering cannot hide a shortened recording. Guided scans warn when packet
timestamps cover over two seconds less than requested. A quiet beginning or end
can also produce a short interval, so this remains a diagnostic warning.

The scanner searches bit offsets for supported records. It counts a candidate
only after recognizing a name in the relevant game catalog. Rejections identify
the parser phase: header, progression, equipment reference, main stats, secondary
stats, character state fields, skill array, quantity or trailer.

These are candidate sightings, not confirmed invalid inventory objects. A
reference to a known object or a partial copy can resemble the beginning of a
record. Raw packets and reconstructed streams can also contain the same object;
accepted sightings therefore differ from the unique records retained.

An unknown catalog identifier is not assigned to a domain speculatively. No
decoded records can mean absent traffic, a missing catalog entry or a changed
wire format. The scanner cannot prove completeness without an independent
expected inventory count. Typed JSON import checks preserve existing validation;
they are not a complete semantic validator for every optional game field.

## Module placement failure localization

Placement import accepts validated cell groups from reconstructed inventory
streams containing the module's character owner, including streams with several
characters. Each module must match the decoded inventory by its exact network
identity, and its inventory owner must match a character identity in both that
stream and the decoded roster. Counting characters in the stream is not an
ownership rule.

Repeated observations must agree on coordinates. Conflicting coordinates are
rejected rather than resolved by stream iteration order. Modules occupying the
same cells on one character's grid are also rejected. Equal coordinates on
different characters' grids are valid.

Diagnostics also examine streams without character records, raw payloads and
completed Unreal messages. Those additional sources do not change imported
placements or infer ownership. Cell groups are never combined across messages.

Each equipped module belongs to exactly one summary category, in this order:

1. An imported placement exists.
2. Eligible inventory streams contain contradictory coordinates for the module.
3. Proposed module cells overlap on the same character's grid.
4. A complete valid group was found only outside the permitted owner association.
5. Cell records were recovered, but no complete group passed validation.
6. A candidate matching the known module was recognized, but its cell fields
   did not pass validation.
7. No candidate matching the supported format was recognized.

The final category cannot distinguish a different wire format from data that
was never captured. The other categories provide positive evidence that the
decoder encountered a supported prefix or cell structure. Candidate recognition
alone remains weaker evidence than a complete validated group.

Rejection counters distinguish truncated fields, grid bounds, unsupported flags,
item mismatches, duplicate cells, multiple origins, absent origins, missing
catalog geometry, inconsistent cell counts and shape mismatches. Source counters
show valid group sightings in eligible streams, excluded streams, raw payloads
and completed messages. Groups are never merged across unrelated sources;
sighting counts can include duplicates, while module categories count unique
equipped modules.

## Furniture failure localization

The furniture diagnostic distinguishes:

1. No completed channel 4 message.
2. Channel 4 present, but no literal `FurnitureActivateStates` property name.
   A name reference encoded differently remains possible.
3. Property name present, but its length, suffix, byte-array size or available
   payload is incompatible with the supported envelope.
4. A bounded payload was extracted, but LZ4 decoding failed or exceeded the
   decompression limit.
5. LZ4 succeeded, but the furniture count, identifier uniqueness, record fields,
   level range or final trailer did not match the supported structure.

Maximum compressed and decoded byte sizes are recorded. Existing bounds remain
8,192 compressed bytes, 65,536 decoded bytes and 256 furniture observations.
The decoder accepts both plain string identifiers with byte-sized flags and
packed records with dynamic names and one-bit flags.
The last fully valid furniture payload remains authoritative; a subsequent
rejected candidate does not overwrite it.

## Privacy and reproduction

Diagnostic events contain fixed codes, timestamps and aggregate integer counts.
They exclude packet bytes, compressed strings, decoded values, item identities,
account identifiers, addresses, paths and raw error messages. Import failures
use fixed JSON/type/file/validation codes instead of copying the original error.
JSON syntax/type failures include their byte offset; validation failures can
include the number of records checked before rejection.

An undecoded block cannot be reliably anonymized: its contents are unknown.
Base64 and hexadecimal encoding do not remove personal data. Raw payloads are
therefore excluded from the shareable log, including rejected channel 4 blocks.
The existing developer `-dump-udp` option produces private raw research files;
it is not enabled by the guided scan and its output must not be treated as a
sanitized diagnostic log.

## Retaining a private capture

In the Import page, expand **Diagnostic options** and explicitly select **Keep
this scan's private packet capture** before starting a normal or recovery scan.
The option starts unchecked and does not extend the selected capture window.
It enables the helper's `-retain-capture` flag, which requires `-login-capture`.

After conversion and before decoding, the helper copies the PCAPNG into
`%LOCALAPPDATA%\NTE Optimizer\workspace\diagnostic-captures`. Each scan gets a
separate file. Copies are streamed to disk with a 1 GiB bound. The scanner can
replay a source of that size incrementally while keeping at most 256 MiB of
selected packet bytes in memory. Source and selected-data limits have distinct
fixed diagnostic codes. ETL
files and temporary PCAPNG files keep their normal cleanup behavior. Retained
copies survive decoding failures, input cleanup and later scans; remove them
manually when the investigation ends. Nothing is uploaded automatically.

The shareable log reports retention success/failure and byte count without the
capture path or contents. These PCAPNG files contain private raw traffic and
must not be attached to public issues or committed. Keep the capture local for
offline investigation.

A retained PCAPNG can be replayed without administrator rights or logging in
again. From the installation directory, the following command decodes the most
recent retained capture using the installed catalogs. It creates a separate
diagnostic log without importing or replacing the current account snapshot:

```powershell
$retainedDirectory = Join-Path $env:LOCALAPPDATA 'NTE Optimizer\workspace\diagnostic-captures'
$retainedFile = Get-ChildItem -LiteralPath $retainedDirectory -Filter '*.pcapng' -File |
    Sort-Object LastWriteTime -Descending | Select-Object -First 1
if (-not $retainedFile) { throw 'No retained capture exists.' }
$replayDirectory = Join-Path ([IO.Path]::GetTempPath()) ('nte-replay-' + [guid]::NewGuid())
New-Item -ItemType Directory -Path $replayDirectory | Out-Null
$replayLog = Join-Path $replayDirectory 'scan-diagnostic.jsonl'
New-Item -ItemType File -Path $replayLog | Out-Null
& .\nte-scan.exe -input $retainedFile.FullName -scan-log $replayLog
```

Only `scan-diagnostic.jsonl` is designed for sharing. The helper's normal
console output and optional raw exports are separate private research data.

Synthetic scanner tests cover absent properties, unaligned envelopes, truncated
arrays, oversized payloads, invalid LZ4, rejected furniture levels and record
truncation across domains. Import tests cover malformed JSON, wrong JSON types,
unsupported versions, duplicate character identities and catalog mismatches.
These reproduce failure classes without requiring real user captures; they do
not establish the exact cause of an individual report.

Run focused diagnostics tests with:

```powershell
go test ./cmd/nte-scan ./internal/decoded ./internal/app ./internal/scanlog
```
