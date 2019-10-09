#include "sph_skein.h"
#include "sph_cubehash.h"
#include "sph_types.h"
#include "sph_fugue.h"
#include "sph_gost.h"

/* ----------- Skunk Hash ------------------------------------------------ */
void skunk_hash(void* input, void* output);
void skunk_midstate(void* input, void* output);
