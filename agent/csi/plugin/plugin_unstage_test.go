package plugin

import (
	"context"
	"testing"

	"github.com/moby/swarmkit/v2/api"
	"github.com/moby/swarmkit/v2/testutils"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"google.golang.org/grpc/codes"
)

func newUnstageTestAssignment(plugin string) *api.VolumeAssignment {
	return &api.VolumeAssignment{
		ID:       "volumeid5",
		VolumeID: "vol5",
		AccessMode: &api.VolumeAccessMode{
			Scope:   api.VolumeScopeSingleNode,
			Sharing: api.VolumeSharingOneWriter,
			AccessType: &api.VolumeAccessMode_Mount{
				Mount: &api.VolumeAccessMode_MountVolume{},
			},
		},
		Driver: &api.Driver{
			Name: plugin,
		},
	}
}

// TestNodeUnstageVolumeCallsDriver checks that a volume which was staged,
// published and then unpublished through the plugin is actually unstaged by
// the CSI driver, and is no longer tracked afterwards. Previously, the plugin
// returned early for any tracked volume, so NodeUnstageVolume never reached
// the driver and the staging mount was leaked while the manager went on to
// detach the volume.
func TestNodeUnstageVolumeCallsDriver(t *testing.T) {
	ctx := context.Background()
	nodePlugin := newVolumeClient("plugin-5", "node-1")
	fakeClient := nodePlugin.nodeClient.(*fakeNodeClient)
	s := newUnstageTestAssignment("plugin-5")

	require.NoError(t, nodePlugin.NodeStageVolume(ctx, s))
	require.NoError(t, nodePlugin.NodePublishVolume(ctx, s))
	require.NoError(t, nodePlugin.NodeUnpublishVolume(ctx, s))
	require.NoError(t, nodePlugin.NodeUnstageVolume(ctx, s))

	require.Len(t, fakeClient.unstageVolumeRequests, 1)
	assert.Equal(t, s.VolumeID, fakeClient.unstageVolumeRequests[0].VolumeId)
	assert.Equal(t, stagePath(s), fakeClient.unstageVolumeRequests[0].StagingTargetPath)
	assert.NotContains(t, fakeClient.stagedVolumes, s.VolumeID)
	assert.NotContains(t, nodePlugin.volumeMap, s.ID)
}

// TestNodeUnstageVolumeStillPublished checks that a volume cannot be unstaged
// while it is still published, and that the driver is not called in that case.
func TestNodeUnstageVolumeStillPublished(t *testing.T) {
	ctx := context.Background()
	nodePlugin := newVolumeClient("plugin-6", "node-1")
	fakeClient := nodePlugin.nodeClient.(*fakeNodeClient)
	s := newUnstageTestAssignment("plugin-6")

	require.NoError(t, nodePlugin.NodeStageVolume(ctx, s))
	require.NoError(t, nodePlugin.NodePublishVolume(ctx, s))

	err := nodePlugin.NodeUnstageVolume(ctx, s)
	assert.Equal(t, codes.FailedPrecondition, testutils.ErrorCode(err))
	assert.Empty(t, fakeClient.unstageVolumeRequests)
	assert.Contains(t, fakeClient.stagedVolumes, s.VolumeID)
	assert.Contains(t, nodePlugin.volumeMap, s.ID)
	assert.NotEmpty(t, nodePlugin.GetPublishedPath(s.ID))
}

// TestNodeUnstageVolumeUntracked checks that a volume which is staged in the
// driver but is not tracked by the plugin (for example, because the agent was
// restarted and lost its in-memory state) is still unstaged by the driver.
func TestNodeUnstageVolumeUntracked(t *testing.T) {
	ctx := context.Background()
	nodePlugin := newVolumeClient("plugin-7", "node-1")
	fakeClient := nodePlugin.nodeClient.(*fakeNodeClient)
	s := newUnstageTestAssignment("plugin-7")

	// staged in the driver, but unknown to this plugin instance
	fakeClient.stagedVolumes[s.VolumeID] = struct{}{}
	require.NotContains(t, nodePlugin.volumeMap, s.ID)

	require.NoError(t, nodePlugin.NodeUnpublishVolume(ctx, s))
	require.NoError(t, nodePlugin.NodeUnstageVolume(ctx, s))

	require.Len(t, fakeClient.unstageVolumeRequests, 1)
	assert.Equal(t, s.VolumeID, fakeClient.unstageVolumeRequests[0].VolumeId)
	assert.Equal(t, stagePath(s), fakeClient.unstageVolumeRequests[0].StagingTargetPath)
	assert.NotContains(t, fakeClient.stagedVolumes, s.VolumeID)
	assert.NotContains(t, nodePlugin.volumeMap, s.ID)
}
