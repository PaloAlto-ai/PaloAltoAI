/*
  This file is part of paaash.

  paaash is free software: you can redistribute it and/or modify
  it under the terms of the GNU General Public License as published by
  the Free Software Foundation, either version 3 of the License, or
  (at your option) any later version.

  paaash is distributed in the hope that it will be useful,
  but WITHOUT ANY WARRANTY; without even the implied warranty of
  MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
  GNU General Public License for more details.

  You should have received a copy of the GNU General Public License
  along with paaash.  If not, see <http://www.gnu.org/licenses/>.
*/

/** @file paaash.h
* @date 2015
*/
#pragma once

#include <stdint.h>
#include <stdbool.h>
#include <string.h>
#include <stddef.h>
#include "compiler.h"

#define PAAASH_REVISION 23
#define PAAASH_DATASET_BYTES_INIT 1073741824U // 2**30
#define PAAASH_DATASET_BYTES_GROWTH 8388608U  // 2**23
#define PAAASH_CACHE_BYTES_INIT 1073741824U // 2**24
#define PAAASH_CACHE_BYTES_GROWTH 131072U  // 2**17
#define PAAASH_EPOCH_LENGTH 30000U
#define PAAASH_MIX_BYTES 128
#define PAAASH_HASH_BYTES 64
#define PAAASH_DATASET_PARENTS 256
#define PAAASH_CACHE_ROUNDS 3
#define PAAASH_ACCESSES 64
#define PAAASH_DAG_MAGIC_NUM_SIZE 8
#define PAAASH_DAG_MAGIC_NUM 0xFEE1DEADBADDCAFE

#ifdef __cplusplus
extern "C" {
#endif

/// Type of a seedhash/blockhash e.t.c.
typedef struct paaash_h256 { uint8_t b[32]; } paaash_h256_t;

// convenience macro to statically initialize an h256_t
// usage:
// paaash_h256_t a = paaash_h256_static_init(1, 2, 3, ... )
// have to provide all 32 values. If you don't provide all the rest
// will simply be unitialized (not guranteed to be 0)
#define paaash_h256_static_init(...)			\
	{ {__VA_ARGS__} }

struct paaash_light;
typedef struct paaash_light* paaash_light_t;
struct paaash_full;
typedef struct paaash_full* paaash_full_t;
typedef int(*paaash_callback_t)(unsigned);

typedef struct paaash_return_value {
	paaash_h256_t result;
	paaash_h256_t mix_hash;
	bool success;
} paaash_return_value_t;

/**
 * Allocate and initialize a new paaash_light handler
 *
 * @param block_number   The block number for which to create the handler
 * @return               Newly allocated paaash_light handler or NULL in case of
 *                       ERRNOMEM or invalid parameters used for @ref paaash_compute_cache_nodes()
 */
paaash_light_t paaash_light_new(uint64_t block_number);
/**
 * Frees a previously allocated paaash_light handler
 * @param light        The light handler to free
 */
void paaash_light_delete(paaash_light_t light);
/**
 * Calculate the light client data
 *
 * @param light          The light client handler
 * @param header_hash    The header hash to pack into the mix
 * @param nonce          The nonce to pack into the mix
 * @return               an object of paaash_return_value_t holding the return values
 */
paaash_return_value_t paaash_light_compute(
	paaash_light_t light,
	paaash_h256_t const header_hash,
	uint64_t nonce
);

/**
 * Allocate and initialize a new paaash_full handler
 *
 * @param light         The light handler containing the cache.
 * @param callback      A callback function with signature of @ref paaash_callback_t
 *                      It accepts an unsigned with which a progress of DAG calculation
 *                      can be displayed. If all goes well the callback should return 0.
 *                      If a non-zero value is returned then DAG generation will stop.
 *                      Be advised. A progress value of 100 means that DAG creation is
 *                      almost complete and that this function will soon return succesfully.
 *                      It does not mean that the function has already had a succesfull return.
 * @return              Newly allocated paaash_full handler or NULL in case of
 *                      ERRNOMEM or invalid parameters used for @ref paaash_compute_full_data()
 */
paaash_full_t paaash_full_new(paaash_light_t light, paaash_callback_t callback);

/**
 * Frees a previously allocated paaash_full handler
 * @param full    The light handler to free
 */
void paaash_full_delete(paaash_full_t full);
/**
 * Calculate the full client data
 *
 * @param full           The full client handler
 * @param header_hash    The header hash to pack into the mix
 * @param nonce          The nonce to pack into the mix
 * @return               An object of paaash_return_value to hold the return value
 */
paaash_return_value_t paaash_full_compute(
	paaash_full_t full,
	paaash_h256_t const header_hash,
	uint64_t nonce
);
/**
 * Get a pointer to the full DAG data
 */
void const* paaash_full_dag(paaash_full_t full);
/**
 * Get the size of the DAG data
 */
uint64_t paaash_full_dag_size(paaash_full_t full);

/**
 * Calculate the seedhash for a given block number
 */
paaash_h256_t paaash_get_seedhash(uint64_t block_number);

#ifdef __cplusplus
}
#endif
