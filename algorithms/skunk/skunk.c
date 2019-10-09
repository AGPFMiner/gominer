#include <string.h>
#include "skunk.h"
/* ----------- Skunk Hash ------------------------------------------------ */
void skunk_hash(void* input, void* output)
{
	sph_skein512_context	 ctx_skein;
	sph_cubehash512_context  ctx_cubehash;
	sph_fugue512_context	 ctx_fugue;
	sph_gost512_context 	 ctx_gost;

	uint32_t hash[16];

	sph_skein512_init(&ctx_skein);
	sph_skein512 (&ctx_skein, input, 80);
	sph_skein512_close(&ctx_skein, hash);

	sph_cubehash512_init(&ctx_cubehash);
	sph_cubehash512 (&ctx_cubehash, hash, 64);
	sph_cubehash512_close(&ctx_cubehash, hash);

	sph_fugue512_init(&ctx_fugue);
	sph_fugue512 (&ctx_fugue, hash, 64);
	sph_fugue512_close(&ctx_fugue, hash);

	sph_gost512_init(&ctx_gost);
	sph_gost512 (&ctx_gost, hash, 64);
	sph_gost512_close(&ctx_gost, hash);

	memcpy(output, hash, 32);
}

void skunk_midstate(void* input, void* output)
{
    sph_skein512_context     ctx_skein;
    sph_u64 midstate[10];
    memcpy(midstate, input, 80);
    
    sph_skein512_init(&ctx_skein);
    sph_skein512 (&ctx_skein, input, 80);
    midstate[0] = ctx_skein.h0;
    midstate[1] = ctx_skein.h1;
    midstate[2] = ctx_skein.h2;
    midstate[3] = ctx_skein.h3;
    midstate[4] = ctx_skein.h4;
    midstate[5] = ctx_skein.h5;
    midstate[6] = ctx_skein.h6;
    midstate[7] = ctx_skein.h7;
    
    memcpy(output, midstate, 80);
}
