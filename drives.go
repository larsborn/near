package main

// localFsType reports whether a file system type is a disk file system
// (as opposed to network, pseudo or in-memory ones).
func localFsType(fstype string) bool {
	switch fstype {
	case "ext2", "ext3", "ext4", "xfs", "btrfs", "zfs", "f2fs", "jfs", "reiserfs", "bcachefs",
		"ntfs", "ntfs3", "fuseblk", "vfat", "msdos", "exfat", "hfs", "hfsplus", "apfs", "ufs":
		return true
	}
	return false
}
