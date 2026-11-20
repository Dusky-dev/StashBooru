cmake_minimum_required(VERSION 3.13)
project(runner LANGUAGES CXX)

# Define the application target. To change its name, change BINARY_NAME in the
# top-level CMakeLists.txt, not the value here, or `flutter run` will no longer
# work.
#
# Any new source files that you add to the application should be added here.
add_executable(${BINARY_NAME}
  "main.cc"
  "my_application.cc"
  "${FLUTTER_MANAGED_DIR}/generated_plugin_registrant.cc"
)

# Apply the standard set of build settings. This can be removed for applications
# that need different build settings.
apply_standard_settings(${BINARY_NAME})

# Add preprocessor definitions for the application ID.
add_definitions(-DAPPLICATION_ID="${APPLICATION_ID}")

# libflutter_linux_gtk.so exports its own _Unwind_* functions. If it comes
# before libgcc_s in the global symbol lookup order, an error raised inside a
# LuaJIT-built libmpv's scripts (osc, ytdl_hook, ...) unwinds through the
# wrong unwinder and the app crashes. Linking libgcc_s first, ahead of every
# other library, keeps the system unwinder in charge.
# scripts/check-linux-app-link-order.sh checks the result.
target_link_options(${BINARY_NAME} BEFORE PRIVATE
  "LINKER:--push-state,--no-as-needed" "-lgcc_s" "LINKER:--pop-state")

# Add dependency libraries. Add any application-specific dependencies here.
target_link_libraries(${BINARY_NAME} PRIVATE flutter)
target_link_libraries(${BINARY_NAME} PRIVATE PkgConfig::GTK)

target_include_directories(${BINARY_NAME} PRIVATE "${CMAKE_SOURCE_DIR}")
