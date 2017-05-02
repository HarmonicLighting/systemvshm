package shm

//#include <stdlib.h>
//#include <sys/shm.h>
import "C"
import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"unsafe"

	"golang.org/x/sys/unix"
)

// errorString is a trivial implementation of error.
type errorString struct {
	s string
}

func (e *errorString) Error() string {
	return e.s
}

// VShmHandler Handles the System V shm calls
type VShmHandler struct {
	attached    bool
	filename    string
	permissions uintptr
	projectid   int
	size        uintptr
	key         uintptr
	shmid       uintptr
	ptr         uintptr
}

// vShmMarshalStr is the var type which stores data for and from JSON
type vShmMarshalStr struct {
	Permissions uintptr
	Projectid   int
	Size        uintptr
	Key         uintptr
	Shmid       uintptr
}

func ftok(pathname string, projectID int) (uintptr, error) {
	cpath := C.CString(pathname)
	defer C.free(unsafe.Pointer(cpath))

	key, err := C.ftok(cpath, C.int(projectID))
	if err != nil {
		return 0, err
	}

	return uintptr(key), nil
}

// ToString generates a string representing the inner status of the object.
func (h *VShmHandler) ToString() string {
	return fmt.Sprintf("{\tattached: %v\n\tfilename: %v\n\tpermissions: %o\n\tprojectid: %v\n\tsize:%v\n\tkey: 0x%x\n\tshmid: %v\n\tptr: %v\n}", h.attached, h.filename, h.permissions, h.projectid, h.size, h.key, h.shmid, h.ptr)
}

// CreateShm creates a struct VShmHandler
func (h *VShmHandler) CreateShm(pathname string, size uintptr, permissionsMask uintptr, projectID int) error {
	if h.attached {
		return &errorString{"This handler is already attached."}
	}
	if size < 1 {
		return &errorString{"The size is invalid."}
	}
	h.filename = pathname
	h.permissions = permissionsMask & 0666
	h.projectid = projectID
	h.size = size
	var err error

	err = generateEmptyFile(pathname)
	if err != nil {
		return err
	}

	h.key, err = ftok(pathname, h.projectid)
	if err != nil {
		return &errorString{"ftok Error."}
	}

	tempSHMID, _, er := unix.Syscall(unix.SYS_SHMGET, h.key, h.size, 01000|h.permissions)
	if er != 0 {
		return err
	}
	h.shmid = tempSHMID

	err = h.generateShmJSONFile()
	if err != nil {
		return err
	}
	return nil
}

// AttachShm attaches to a previously created shm
func (h *VShmHandler) AttachShm(pathname string) (uintptr, error) {
	if h.attached {
		return 0, &errorString{"This handler is already attached."}
	}

	datashm, err := readShmJSONFile(pathname)
	if err != nil {
		return 0, err
	}

	addr, _, e := unix.Syscall(unix.SYS_SHMAT, uintptr(datashm.Shmid), 0, 0)
	if e != 0 {
	}
	h.filename = pathname
	h.permissions = datashm.Permissions
	h.projectid = datashm.Projectid
	h.size = datashm.Size
	h.key = datashm.Key
	h.shmid = datashm.Shmid
	h.ptr = addr
	h.attached = true
	return h.ptr, nil
}

// DetachShm Detaches from the shared memory.
func (h *VShmHandler) DetachShm() error {
	if !h.attached {
		return &errorString{"Couldn't detach. This handler wasn't attached."}
	}
	ret, _, err := unix.Syscall(unix.SYS_SHMDT, h.ptr, 0, 0)
	if ret != 0 {
		return err
	}
	h.attached = false
	h.ptr = 0
	return nil
}

// NewVShmHandler returns a VShmHandler
func NewVShmHandler() *VShmHandler {
	return &VShmHandler{false, "", 0, 0, 0, 0, 0, 0}
}

func (h *VShmHandler) generateShmJSONFile() error {

	info := &vShmMarshalStr{
		Permissions: h.permissions,
		Projectid:   h.projectid,
		Size:        h.size,
		Key:         h.key,
		Shmid:       h.shmid,
	}

	bytes, ok := json.Marshal(info)
	if ok != nil {
		return ok
	}
	err := ioutil.WriteFile(h.filename, bytes, os.FileMode(h.permissions))
	if err != nil {
		return err
	}
	return nil
}

func generateEmptyFile(pathname string) error {
	newFile, err := os.Create(pathname)
	if err != nil {
		return err
	}
	newFile.Close()
	return nil
}

func readShmJSONFile(pathname string) (*vShmMarshalStr, error) {
	bytes, err := ioutil.ReadFile(pathname)
	if err != nil {
		return &vShmMarshalStr{}, err
	}
	buffer := &vShmMarshalStr{}
	err = json.Unmarshal(bytes, buffer)
	if err != nil {
		return &vShmMarshalStr{}, err
	}
	return buffer, nil
}
