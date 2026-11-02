package main

// A fragmented property may be larger than a packet, but must fit in the
// decoder's per-block memory budget. Reject overflow before allocating.
const maxReassembledMessageBytes = 16 << 20
