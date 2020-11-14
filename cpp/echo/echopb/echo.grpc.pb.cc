// Generated by the gRPC C++ plugin.
// If you make any local change, they will be lost.
// source: echo.proto

#include "echo.pb.h"
#include "echo.grpc.pb.h"

#include <functional>
#include <grpcpp/impl/codegen/async_stream.h>
#include <grpcpp/impl/codegen/async_unary_call.h>
#include <grpcpp/impl/codegen/channel_interface.h>
#include <grpcpp/impl/codegen/client_unary_call.h>
#include <grpcpp/impl/codegen/client_callback.h>
#include <grpcpp/impl/codegen/message_allocator.h>
#include <grpcpp/impl/codegen/method_handler.h>
#include <grpcpp/impl/codegen/rpc_service_method.h>
#include <grpcpp/impl/codegen/server_callback.h>
#include <grpcpp/impl/codegen/server_callback_handlers.h>
#include <grpcpp/impl/codegen/server_context.h>
#include <grpcpp/impl/codegen/service_type.h>
#include <grpcpp/impl/codegen/sync_stream.h>
namespace echo {

static const char* EchoService_method_names[] = {
  "/echo.EchoService/Stop",
  "/echo.EchoService/Echo",
};

std::unique_ptr< EchoService::Stub> EchoService::NewStub(const std::shared_ptr< ::grpc::ChannelInterface>& channel, const ::grpc::StubOptions& options) {
  (void)options;
  std::unique_ptr< EchoService::Stub> stub(new EchoService::Stub(channel));
  return stub;
}

EchoService::Stub::Stub(const std::shared_ptr< ::grpc::ChannelInterface>& channel)
  : channel_(channel), rpcmethod_Stop_(EchoService_method_names[0], ::grpc::internal::RpcMethod::NORMAL_RPC, channel)
  , rpcmethod_Echo_(EchoService_method_names[1], ::grpc::internal::RpcMethod::NORMAL_RPC, channel)
  {}

::grpc::Status EchoService::Stub::Stop(::grpc::ClientContext* context, const ::echo::StopRequest& request, ::echo::StopResponse* response) {
  return ::grpc::internal::BlockingUnaryCall(channel_.get(), rpcmethod_Stop_, context, request, response);
}

void EchoService::Stub::experimental_async::Stop(::grpc::ClientContext* context, const ::echo::StopRequest* request, ::echo::StopResponse* response, std::function<void(::grpc::Status)> f) {
  ::grpc_impl::internal::CallbackUnaryCall(stub_->channel_.get(), stub_->rpcmethod_Stop_, context, request, response, std::move(f));
}

void EchoService::Stub::experimental_async::Stop(::grpc::ClientContext* context, const ::grpc::ByteBuffer* request, ::echo::StopResponse* response, std::function<void(::grpc::Status)> f) {
  ::grpc_impl::internal::CallbackUnaryCall(stub_->channel_.get(), stub_->rpcmethod_Stop_, context, request, response, std::move(f));
}

void EchoService::Stub::experimental_async::Stop(::grpc::ClientContext* context, const ::echo::StopRequest* request, ::echo::StopResponse* response, ::grpc::experimental::ClientUnaryReactor* reactor) {
  ::grpc_impl::internal::ClientCallbackUnaryFactory::Create(stub_->channel_.get(), stub_->rpcmethod_Stop_, context, request, response, reactor);
}

void EchoService::Stub::experimental_async::Stop(::grpc::ClientContext* context, const ::grpc::ByteBuffer* request, ::echo::StopResponse* response, ::grpc::experimental::ClientUnaryReactor* reactor) {
  ::grpc_impl::internal::ClientCallbackUnaryFactory::Create(stub_->channel_.get(), stub_->rpcmethod_Stop_, context, request, response, reactor);
}

::grpc::ClientAsyncResponseReader< ::echo::StopResponse>* EchoService::Stub::AsyncStopRaw(::grpc::ClientContext* context, const ::echo::StopRequest& request, ::grpc::CompletionQueue* cq) {
  return ::grpc_impl::internal::ClientAsyncResponseReaderFactory< ::echo::StopResponse>::Create(channel_.get(), cq, rpcmethod_Stop_, context, request, true);
}

::grpc::ClientAsyncResponseReader< ::echo::StopResponse>* EchoService::Stub::PrepareAsyncStopRaw(::grpc::ClientContext* context, const ::echo::StopRequest& request, ::grpc::CompletionQueue* cq) {
  return ::grpc_impl::internal::ClientAsyncResponseReaderFactory< ::echo::StopResponse>::Create(channel_.get(), cq, rpcmethod_Stop_, context, request, false);
}

::grpc::Status EchoService::Stub::Echo(::grpc::ClientContext* context, const ::echo::EchoRequest& request, ::echo::EchoResponse* response) {
  return ::grpc::internal::BlockingUnaryCall(channel_.get(), rpcmethod_Echo_, context, request, response);
}

void EchoService::Stub::experimental_async::Echo(::grpc::ClientContext* context, const ::echo::EchoRequest* request, ::echo::EchoResponse* response, std::function<void(::grpc::Status)> f) {
  ::grpc_impl::internal::CallbackUnaryCall(stub_->channel_.get(), stub_->rpcmethod_Echo_, context, request, response, std::move(f));
}

void EchoService::Stub::experimental_async::Echo(::grpc::ClientContext* context, const ::grpc::ByteBuffer* request, ::echo::EchoResponse* response, std::function<void(::grpc::Status)> f) {
  ::grpc_impl::internal::CallbackUnaryCall(stub_->channel_.get(), stub_->rpcmethod_Echo_, context, request, response, std::move(f));
}

void EchoService::Stub::experimental_async::Echo(::grpc::ClientContext* context, const ::echo::EchoRequest* request, ::echo::EchoResponse* response, ::grpc::experimental::ClientUnaryReactor* reactor) {
  ::grpc_impl::internal::ClientCallbackUnaryFactory::Create(stub_->channel_.get(), stub_->rpcmethod_Echo_, context, request, response, reactor);
}

void EchoService::Stub::experimental_async::Echo(::grpc::ClientContext* context, const ::grpc::ByteBuffer* request, ::echo::EchoResponse* response, ::grpc::experimental::ClientUnaryReactor* reactor) {
  ::grpc_impl::internal::ClientCallbackUnaryFactory::Create(stub_->channel_.get(), stub_->rpcmethod_Echo_, context, request, response, reactor);
}

::grpc::ClientAsyncResponseReader< ::echo::EchoResponse>* EchoService::Stub::AsyncEchoRaw(::grpc::ClientContext* context, const ::echo::EchoRequest& request, ::grpc::CompletionQueue* cq) {
  return ::grpc_impl::internal::ClientAsyncResponseReaderFactory< ::echo::EchoResponse>::Create(channel_.get(), cq, rpcmethod_Echo_, context, request, true);
}

::grpc::ClientAsyncResponseReader< ::echo::EchoResponse>* EchoService::Stub::PrepareAsyncEchoRaw(::grpc::ClientContext* context, const ::echo::EchoRequest& request, ::grpc::CompletionQueue* cq) {
  return ::grpc_impl::internal::ClientAsyncResponseReaderFactory< ::echo::EchoResponse>::Create(channel_.get(), cq, rpcmethod_Echo_, context, request, false);
}

EchoService::Service::Service() {
  AddMethod(new ::grpc::internal::RpcServiceMethod(
      EchoService_method_names[0],
      ::grpc::internal::RpcMethod::NORMAL_RPC,
      new ::grpc::internal::RpcMethodHandler< EchoService::Service, ::echo::StopRequest, ::echo::StopResponse>(
          [](EchoService::Service* service,
             ::grpc::ServerContext* ctx,
             const ::echo::StopRequest* req,
             ::echo::StopResponse* resp) {
               return service->Stop(ctx, req, resp);
             }, this)));
  AddMethod(new ::grpc::internal::RpcServiceMethod(
      EchoService_method_names[1],
      ::grpc::internal::RpcMethod::NORMAL_RPC,
      new ::grpc::internal::RpcMethodHandler< EchoService::Service, ::echo::EchoRequest, ::echo::EchoResponse>(
          [](EchoService::Service* service,
             ::grpc::ServerContext* ctx,
             const ::echo::EchoRequest* req,
             ::echo::EchoResponse* resp) {
               return service->Echo(ctx, req, resp);
             }, this)));
}

EchoService::Service::~Service() {
}

::grpc::Status EchoService::Service::Stop(::grpc::ServerContext* context, const ::echo::StopRequest* request, ::echo::StopResponse* response) {
  (void) context;
  (void) request;
  (void) response;
  return ::grpc::Status(::grpc::StatusCode::UNIMPLEMENTED, "");
}

::grpc::Status EchoService::Service::Echo(::grpc::ServerContext* context, const ::echo::EchoRequest* request, ::echo::EchoResponse* response) {
  (void) context;
  (void) request;
  (void) response;
  return ::grpc::Status(::grpc::StatusCode::UNIMPLEMENTED, "");
}


}  // namespace echo
