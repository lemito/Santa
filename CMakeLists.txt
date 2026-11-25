cmake_minimum_required(VERSION 3.20)
project(santa VERSION 1.0.0 LANGUAGES C CXX)
include(GNUInstallDirs)
include(CMakePackageConfigHelpers)
set(CMAKE_CXX_STANDARD 20)
set(CMAKE_CXX_STANDARD_REQUIRED ON)
set(CMAKE_CXX_EXTENSIONS OFF)
set(CMAKE_POSITION_INDEPENDENT_CODE ON)
option(SANTA_SANITIZE "Enable address and undefined behavior sanitizers" OFF)
option(SANTA_COVERAGE "Enable LLVM/GCC coverage instrumentation" OFF)
if(SANTA_SANITIZE)
  add_compile_options(-fsanitize=address,undefined -fno-omit-frame-pointer)
  add_link_options(-fsanitize=address,undefined)
endif()
if(SANTA_COVERAGE)
  add_compile_options(--coverage -O0 -g)
  add_link_options(--coverage)
endif()
set(YASL_BUILD_TESTS OFF CACHE BOOL "" FORCE)
set(YASL_MALLOC libc CACHE STRING "" FORCE)
add_subdirectory(third_party/yasl EXCLUDE_FROM_ALL)
function(santa_strict target)
  target_compile_options(${target} PRIVATE
    $<$<COMPILE_LANGUAGE:CXX>:-Wall;-Wextra;-Wpedantic;-Werror>
    $<$<COMPILE_LANGUAGE:C>:-Wall;-Wextra;-Werror>)
endfunction()
if(CMAKE_SYSTEM_NAME STREQUAL "Linux")
  set(SANTA_RANDOM_SOURCE core/src/random_linux.cpp)
else()
  message(FATAL_ERROR "Santa native RNG is currently supported only on Linux")
endif()
add_library(santa_core STATIC core/src/engine.cpp ${SANTA_RANDOM_SOURCE})
target_include_directories(santa_core PUBLIC core/include)
target_link_libraries(santa_core PUBLIC yasl::yasl)
santa_strict(santa_core)
set_target_properties(santa_core PROPERTIES CXX_VISIBILITY_PRESET hidden VISIBILITY_INLINES_HIDDEN YES)
add_library(santa_native SHARED core/src/abi.cpp)
target_link_libraries(santa_native PRIVATE santa_core)
target_include_directories(santa_native PUBLIC
  $<BUILD_INTERFACE:${CMAKE_CURRENT_SOURCE_DIR}/core/include>
  $<INSTALL_INTERFACE:${CMAKE_INSTALL_INCLUDEDIR}>)
santa_strict(santa_native)
set_target_properties(santa_native PROPERTIES
  CXX_VISIBILITY_PRESET hidden
  VISIBILITY_INLINES_HIDDEN YES
  VERSION ${PROJECT_VERSION}
  SOVERSION 1)
include(CTest)
if(BUILD_TESTING)
  add_executable(santa_core_tests core/tests/engine_test.cpp)
  target_link_libraries(santa_core_tests PRIVATE santa_core santa_native)
  santa_strict(santa_core_tests)
  add_test(NAME santa_core_tests COMMAND santa_core_tests)
  add_executable(santa_c_abi_test core/tests/abi_test.c)
  target_link_libraries(santa_c_abi_test PRIVATE santa_native)
  santa_strict(santa_c_abi_test)
  add_test(NAME santa_c_abi_test COMMAND santa_c_abi_test)
  add_executable(santa_abi_fuzz_smoke core/tests/abi_fuzz.cpp)
  target_link_libraries(santa_abi_fuzz_smoke PRIVATE santa_native)
  santa_strict(santa_abi_fuzz_smoke)
  add_test(NAME santa_abi_fuzz_smoke COMMAND santa_abi_fuzz_smoke)
endif()
install(TARGETS santa_native
  EXPORT SantaNativeTargets
  LIBRARY DESTINATION ${CMAKE_INSTALL_LIBDIR}
  ARCHIVE DESTINATION ${CMAKE_INSTALL_LIBDIR}
  RUNTIME DESTINATION ${CMAKE_INSTALL_BINDIR})
install(FILES core/include/santa/abi.h
  DESTINATION ${CMAKE_INSTALL_INCLUDEDIR}/santa)
install(EXPORT SantaNativeTargets
  FILE SantaNativeTargets.cmake
  NAMESPACE Santa::
  DESTINATION ${CMAKE_INSTALL_LIBDIR}/cmake/SantaNative)
configure_package_config_file(
  cmake/SantaNativeConfig.cmake.in
  ${CMAKE_CURRENT_BINARY_DIR}/SantaNativeConfig.cmake
  INSTALL_DESTINATION ${CMAKE_INSTALL_LIBDIR}/cmake/SantaNative)
write_basic_package_version_file(
  ${CMAKE_CURRENT_BINARY_DIR}/SantaNativeConfigVersion.cmake
  VERSION ${PROJECT_VERSION}
  COMPATIBILITY SameMajorVersion)
install(FILES
  ${CMAKE_CURRENT_BINARY_DIR}/SantaNativeConfig.cmake
  ${CMAKE_CURRENT_BINARY_DIR}/SantaNativeConfigVersion.cmake
  DESTINATION ${CMAKE_INSTALL_LIBDIR}/cmake/SantaNative)
configure_file(cmake/santa_native.pc.in
  ${CMAKE_CURRENT_BINARY_DIR}/santa_native.pc @ONLY)
install(FILES ${CMAKE_CURRENT_BINARY_DIR}/santa_native.pc
  DESTINATION ${CMAKE_INSTALL_LIBDIR}/pkgconfig)
add_executable(santa_benchmark EXCLUDE_FROM_ALL core/tests/benchmark.cpp)
target_link_libraries(santa_benchmark PRIVATE santa_core)
santa_strict(santa_benchmark)
