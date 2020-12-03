TEMPLATE = app
CONFIG += console c++17
CONFIG -= app_bundle
CONFIG -= qt

SOURCES += \
    ../../../GlobularClient/globularclient.cpp \
    ../../../GlobularServer/globularserver.cpp \
    ../../../resource/GlobularResourceClient/globularresourceclient.cpp \
    ../../../resource/resourcepb/resource.grpc.pb.cc \
    ../../../resource/resourcepb/resource.pb.cc \
    ../../plcpb/plc.grpc.pb.cc \
    ../../plcpb/plc.pb.cc \
    ../PLC_server/PlcServiceImpl.cpp \
    ../PLC_server/main.cpp

HEADERS += \
    ../../../GlobularClient/globularclient.h \
    ../../../GlobularServer/globularserver.h \
    ../../../resource/GlobularResourceClient/globularresourceclient.h \
    ../../../resource/resourcepb/resource.grpc.pb.h \
    ../../../resource/resourcepb/resource.pb.h \
    ../../plcpb/plc.grpc.pb.h \
    ../../plcpb/plc.pb.h \
    ../PLC_server/PlcServiceImpl.h

INCLUDEPATH += ../../../ ../../plcpb  ../../../resource/resourcepb/ ../../../GlobularClient ../../../resource/GlobularResourceClient


LIBS += `pkg-config --libs grpc++ protobuf`

DISTFILES += \
    ../PLC_server/PLC_server.vcxproj

unix:!macx: LIBS += -lplctag
