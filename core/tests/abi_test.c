#include <assert.h>
#include <santa/abi.h>
#include <stdint.h>

int main(void) {
  SantaBudget budget = {10000, 1000000, 1048576};
  uint32_t output[2] = {99, 99};
  if (sizeof(SantaPair) != 8 || sizeof(SantaBudget) != 24 ||
      santa_abi_version() != SANTA_ABI_VERSION)
    return 1;
  assert(sizeof(uint32_t) == 4);
  if (santa_draw(2, 0, 0, budget, output, 2) != SANTA_SUCCESS)
    return 2;
  if (output[0] != 1 || output[1] != 0)
    return 3;
  output[0] = output[1] = 77;
  if (santa_draw(1, 0, 0, budget, output, 2) != SANTA_INVALID_INPUT ||
      output[0] != 77 || output[1] != 77)
    return 4;
  if (santa_draw(2, 0, 0, budget, output, 1) != SANTA_INVALID_INPUT ||
      output[0] != 77 || output[1] != 77)
    return 5;
  if (santa_draw(2, 0, 1, budget, output, 2) != SANTA_INVALID_INPUT ||
      output[0] != 77 || output[1] != 77)
    return 6;
  SantaPair self = {0, 0};
  if (santa_draw(2, &self, 1, budget, output, 2) != SANTA_INVALID_INPUT ||
      output[0] != 77 || output[1] != 77)
    return 7;
  SantaPair impossible[] = {{0, 1}, {1, 0}};
  if (santa_draw(2, impossible, 2, budget, output, 2) != SANTA_UNSATISFIABLE ||
      output[0] != 77 || output[1] != 77)
    return 8;
  uint32_t cancelled_flag = 1;
  SantaCancellation cancellation = {&cancelled_flag};
  output[0] = output[1] = 66;
  if (santa_draw_ex(2, 0, 0, budget, &cancellation, output, 2) != SANTA_CANCELLED ||
      output[0] != 66 || output[1] != 66)
    return 9;
  return 0;
}
