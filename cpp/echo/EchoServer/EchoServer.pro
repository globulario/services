TEMPLATE = app
CONFIG += console c++
CONFIG -= app_bundle
CONFIG -= qt
CONFIG += c++17

SOURCES += \
        ../../GlobularClient/globularclient.cpp \
        ../../GlobularServer/globularserver.cpp \
        ../../config.cc \
        ../../config/GlobularConfigClient/globular_config_client.cpp \
        ../../config/configpb/config.grpc.pb.cc \
        ../../config/configpb/config.pb.cc \
        ../../resource/GlobularResourceClient/globularresourceclient.cpp \
        ../../resource/resourcepb/resource.grpc.pb.cc \
        ../../resource/resourcepb/resource.pb.cc \
        ../echopb/echo.grpc.pb.cc \
        ../echopb/echo.pb.cc \
        echoserviceimpl.cpp \
        main.cpp

HEADERS += \
    ../../GlobularClient/globularclient.h \
    ../../GlobularServer/globularserver.h \
    ../../config.h \
    ../../config/GlobularConfigClient/globular_config_client.h \
    ../../config/configpb/config.grpc.pb.h \
    ../../config/configpb/config.pb.h \
    ../../resource/GlobularResourceClient/globularresourceclient.h \
    ../../resource/resourcepb/resource.grpc.pb.h \
    ../../resource/resourcepb/resource.pb.h \
    ../echopb/echo.grpc.pb.h \
    ../echopb/echo.pb.h \
    echoserviceimpl.h

INCLUDEPATH +=  ../../ ../../resource ../echopb ../../GlobularServer ../../GlobularClient ../../resource/GlobularResourceClient ../../resource/resourcepb ../config/GlobularConfigClient ../../config/configpb ../../config

#here I will make use of pkg-config to get the list of dependencie of each libraries.
unix: LIBS += `pkg-config --libs grpc++ protobuf`

# Set the pkconfig.
win32: LIBS += -LC:/msys64/mingw64/lib -lgrpc++ -labsl_raw_hash_set -labsl_hashtablez_sampler -labsl_exponential_biased -labsl_hash -labsl_bad_variant_access -labsl_city -labsl_status -labsl_cord -labsl_bad_optional_access -labsl_str_format_internal -labsl_synchronization -labsl_graphcycles_internal -labsl_symbolize -labsl_demangle_internal -labsl_stacktrace -labsl_debugging_internal -labsl_malloc_internal -labsl_time -labsl_time_zone -labsl_civil_time -labsl_strings -labsl_strings_internal -labsl_throw_delegate -labsl_int128 -labsl_base -labsl_spinlock_wait -labsl_raw_logging_internal -labsl_log_severity -lgrpc -laddress_sorting -lre2 -lupb -lcares -lz -labsl_raw_hash_set -labsl_hashtablez_sampler -labsl_exponential_biased -labsl_hash -labsl_bad_variant_access -labsl_city -labsl_status -labsl_cord -labsl_bad_optional_access -labsl_str_format_internal -labsl_synchronization -labsl_graphcycles_internal -labsl_symbolize -labsl_demangle_internal -labsl_stacktrace -labsl_debugging_internal -labsl_malloc_internal -labsl_time -labsl_time_zone -labsl_civil_time -labsl_strings -labsl_strings_internal -labsl_throw_delegate -labsl_int128 -labsl_base -labsl_spinlock_wait -labsl_raw_logging_internal -labsl_log_severity -lgpr -labsl_str_format_internal -labsl_synchronization -labsl_graphcycles_internal -labsl_symbolize -labsl_demangle_internal -labsl_stacktrace -labsl_debugging_internal -labsl_malloc_internal -labsl_time -labsl_time_zone -labsl_civil_time -labsl_strings -labsl_strings_internal -labsl_throw_delegate -labsl_int128 -labsl_base -labsl_spinlock_wait -labsl_raw_logging_internal -labsl_log_severity  -lssl -lcrypto -lws2_32 -lgdi32 -lcrypt32  -limagehlp -lprotobuf
