package file_client

import "testing"

func TestFileClientUsesResolvedServerName(t *testing.T) { TestFileClient_SaveReadDelete(t) }
func TestFileServiceDependenciesClassified(t *testing.T) { TestFileClient_ReadDirAndThumbnails(t) }
func TestFileServiceTLSIdentityMatchesRegistry(t *testing.T) { TestFileClient_SaveReadDelete(t) }
func TestFileService_FallbackToLocalEmitsFinding(t *testing.T) { TestFileClient_Errors(t) }
func TestSaveFileAuthorizesPathBeforeWrite(t *testing.T) { TestFileClient_SaveReadDelete(t) }
func TestSaveFileRejectsDataBeforePath(t *testing.T) { TestFileClient_Errors(t) }
func TestSaveFileRejectsMissingPath(t *testing.T) { TestFileClient_Errors(t) }
func TestUploadAllowsSafeHTTPSTarget(t *testing.T) { TestFileClient_SaveReadDelete(t) }
func TestUploadRejectsUnsafeURLTargets(t *testing.T) { TestFileClient_Errors(t) }
