QT -= gui

TEMPLATE = lib
CONFIG += staticlib

CONFIG += c++17

# The following define makes your compiler emit warnings if you use
# any Qt feature that has been marked deprecated (the exact warnings
# depend on your compiler). Please consult the documentation of the
# deprecated API in order to know how to port your code away from it.
DEFINES += QT_DEPRECATED_WARNINGS

# You can also make your code fail to compile if it uses deprecated APIs.
# In order to do so, uncomment the following line.
# You can also select to disable deprecated APIs only up to a certain version of Qt.
#DEFINES += QT_DISABLE_DEPRECATED_BEFORE=0x060000    # disables all the APIs deprecated before Qt 6.0.0

SOURCES += \
    ../GlobularClient/globularclient.cpp \
    ../resource/GlobularResourceClient/globularresourceclient.cpp \
    ../config/GlobularConfigClient/globular_config_client.cpp \
    ../rbac/GlobularRbacClient/globular_rbac_client.cpp \
    ../resource/resourcepb/resource.grpc.pb.cc \
    ../resource/resourcepb/resource.pb.cc \
    ../rbac/rbacpb/rbac.grpc.pb.cc \
    ../rbac/rbacpb/rbac.pb.cc \
    ../config/configpb/config.grpc.pb.cc \
    ../config/configpb/config.pb.cc \
    globularserver.cpp

HEADERS += \
    ../GlobularClient/globularclient.h \
    ../config/GlobularConfigClient/globular_config_client.h \
    ../rbac/GlobularRbacClient/globular_rbac_client.h \
    ../resource/GlobularResourceClient/globularresourceclient.h \
    ../resource/resourcepb/resource.grpc.pb.h \
    ../resource/resourcepb/resource.pb.h \
    ../rbac/rbacpb/rbac.grpc.pb.h \
    ../rbac/rbacpb/rbac.pb.h \
    ../config/configpb/rbac.grpc.pb.h \
    ../config/configpb/config.pb.h \
    globularserver.h \
    json.hpp

# Default rules for deployment.
unix {
    target.path = $$[QT_INSTALL_PLUGINS]/generic
}
!isEmpty(target.path): INSTALLS += target

INCLUDEPATH += $$PWD/../resource/GlobularResourceClient $$PWD/../GlobularClient ../
