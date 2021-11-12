TEMPLATE = app
CONFIG += console c++17
CONFIG -= app_bundle
QT       += core
DEFINES += GRPC_CALLBACK_API_NONEXPERIMENTAL
SOURCES += \
    ../../GlobularClient/globularclient.cpp \
    ../../GlobularServer/globularserver.cpp \
    ../../Resource/GlobularResourceClient/globularResourceclient.cpp \
    ../../resource/resourcepb/resource.grpc.pb.cc \
    ../../resource/resourcepb/resource.pb.cc \
    ../spcpb/spc.grpc.pb.cc \
    ../spcpb/spc.pb.cc \
    AnalyseurCSP.cpp \
    DonneesAnalyse.cpp \
    Erreur.cpp \
    main.cpp \
    SousGroupe.cpp

HEADERS += \
    ../../GlobularClient/globularclient.h \
    ../../GlobularServer/globularserver.h \
    ../../Resource/GlobularResourceClient/globularResourceclient.h \
    ../../resource/resourcepb/resource.grpc.pb.h \
    ../../resource/resourcepb/resource.pb.h \
    ../spcpb/spc.grpc.pb.h \
    ../spcpb/spc.pb.h \
    AnalyseurCSP.h \
    DonneesAnalyse.h \
    Erreur.h \
    SousGroupe.h


INCLUDEPATH += C:\Users\mm006819\boost_1_74_0 ../../
INCLUDEPATH +=  ../../ ../echopb ../../GlobularServer ../../GlobularClient ../../Resource/GlobularResourceClient ../../Resource/Resourcepb

unix:!macx:INCLUDEPATH += /usr/local/include
win32:INCLUDEPATH += C:/msys64/mingw64/include

#here I will make use of pkg-config to get the list of dependencie of each libraries.
unix: LIBS += `pkg-config --libs grpc++ protobuf`

# Set the pkconfig.
# Static build
#win32: LIBS += -LC:/msys64/mingw64/lib -lgrpc++ -labsl_raw_hash_set -labsl_hashtablez_sampler -labsl_exponential_biased -labsl_hash -labsl_bad_variant_access -labsl_city -labsl_status -labsl_cord -labsl_bad_optional_access -labsl_str_format_internal -labsl_synchronization -labsl_graphcycles_internal -labsl_symbolize -labsl_demangle_internal -labsl_stacktrace -labsl_debugging_internal -labsl_malloc_internal -labsl_time -labsl_time_zone -labsl_civil_time -labsl_strings -labsl_strings_internal -labsl_throw_delegate -labsl_int128 -labsl_base -labsl_spinlock_wait -labsl_raw_logging_internal -labsl_log_severity -lgrpc -laddress_sorting -lre2 -lupb -lcares -lz -labsl_raw_hash_set -labsl_hashtablez_sampler -labsl_exponential_biased -labsl_hash -labsl_bad_variant_access -labsl_city -labsl_status -labsl_cord -labsl_bad_optional_access -labsl_str_format_internal -labsl_synchronization -labsl_graphcycles_internal -labsl_symbolize -labsl_demangle_internal -labsl_stacktrace -labsl_debugging_internal -labsl_malloc_internal -labsl_time -labsl_time_zone -labsl_civil_time -labsl_strings -labsl_strings_internal -labsl_throw_delegate -labsl_int128 -labsl_base -labsl_spinlock_wait -labsl_raw_logging_internal -labsl_log_severity -lgpr -labsl_str_format_internal -labsl_synchronization -labsl_graphcycles_internal -labsl_symbolize -labsl_demangle_internal -labsl_stacktrace -labsl_debugging_internal -labsl_malloc_internal -labsl_time -labsl_time_zone -labsl_civil_time -labsl_strings -labsl_strings_internal -labsl_throw_delegate -labsl_int128 -labsl_base -labsl_spinlock_wait -labsl_raw_logging_internal -labsl_log_severity -lssl -lcrypto -lws2_32 -lgdi32 -lcrypt32  -limagehlp -lprotobuf -lgmp

# Shared library.
#-llibgrpc++.dll
win32: LIBS += -LC:/msys64/mingw64/lib -llibgrpc++.dll  -llibgrpc.dll -llibabsl_raw_hash_set.dll -llibabsl_hashtablez_sampler.dll -llibabsl_exponential_biased.dll -llibabsl_hash.dll -llibabsl_bad_variant_access.dll -llibabsl_city.dll -llibabsl_status.dll -llibabsl_cord.dll -llibabsl_bad_optional_access.dll -llibabsl_str_format_internal.dll -llibabsl_synchronization.dll -llibabsl_graphcycles_internal.dll -llibabsl_symbolize.dll -llibabsl_demangle_internal.dll -llibabsl_stacktrace.dll -llibabsl_debugging_internal.dll -llibabsl_malloc_internal.dll -llibabsl_time.dll -llibabsl_time_zone.dll -llibabsl_civil_time.dll -llibabsl_strings.dll -llibabsl_strings_internal.dll -llibabsl_throw_delegate.dll -llibabsl_int128.dll -llibabsl_base.dll -llibabsl_spinlock_wait.dll -llibabsl_raw_logging_internal.dll -llibabsl_log_severity.dll -llibaddress_sorting.dll -llibre2.dll -llibupb.dll -llibcares.dll -llibz.dll -llibabsl_raw_hash_set.dll -llibabsl_hashtablez_sampler.dll -llibabsl_exponential_biased.dll -llibabsl_hash.dll -llibabsl_bad_variant_access.dll -llibabsl_city.dll -llibabsl_status.dll -llibabsl_cord.dll -llibabsl_bad_optional_access.dll -llibabsl_str_format_internal.dll -llibabsl_synchronization.dll -llibabsl_graphcycles_internal.dll -llibabsl_symbolize.dll -llibabsl_demangle_internal.dll -llibabsl_stacktrace.dll -llibabsl_debugging_internal.dll -llibabsl_malloc_internal.dll -llibabsl_time.dll -llibabsl_time_zone.dll -llibabsl_civil_time.dll -llibabsl_strings.dll -llibabsl_strings_internal.dll -llibabsl_throw_delegate.dll -llibabsl_int128.dll -llibabsl_base.dll -llibabsl_spinlock_wait.dll -llibabsl_raw_logging_internal.dll -llibabsl_log_severity.dll -llibgpr.dll -llibabsl_str_format_internal.dll -llibabsl_synchronization.dll -llibabsl_graphcycles_internal.dll -llibabsl_symbolize.dll -llibabsl_demangle_internal.dll -llibabsl_stacktrace.dll -llibabsl_debugging_internal.dll -llibabsl_malloc_internal.dll -llibabsl_time.dll -llibabsl_time_zone.dll -llibabsl_civil_time.dll -llibabsl_strings.dll -llibabsl_strings_internal.dll -llibabsl_throw_delegate.dll -llibabsl_int128.dll -llibabsl_base.dll -llibabsl_spinlock_wait.dll -llibabsl_raw_logging_internal.dll -llibabsl_log_severity.dll -llibssl.dll -llibcrypto.dll -llibprotobuf.dll -llibgmp.dll


