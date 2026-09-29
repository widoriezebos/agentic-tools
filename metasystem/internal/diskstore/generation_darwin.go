package diskstore

import "os"

// fileGeneration is 0 on Darwin: APFS never reuses an inode number, so the
// device and inode alone tell a replaced .git file apart, and st_gen reads
// as 0 for anyone but root.
func fileGeneration(*os.File) uint64 { return 0 }
