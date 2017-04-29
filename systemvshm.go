package shm

//#include <stdlib.h>
//#include <sys/shm.h>
import "C"
import (
	"encoding/json"
	"fmt"
	"io/ioutil"
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
	attached bool
	filename string
	key      uintptr
	size     uintptr
	shmid    uintptr
	ptr      uintptr
}

// vShmMarshalStr is the var type which stores data for and from JSON
type vShmMarshalStr struct {
	Key   uintptr
	Size  uintptr
	Shmid uintptr
}

func ftok(pathname string) (uintptr, error) {
	cpath := C.CString(pathname)
	defer C.free(unsafe.Pointer(cpath))

	key, err := C.ftok(cpath, C.int(20))
	if err != nil {
		return 0, err
	}

	return uintptr(key), nil
}

// ToString generates a string representing the inner status of the object.
func (h *VShmHandler) ToString() string {
	return fmt.Sprintf("{attached: %v,filename: %v, key: %v, size:%v, shmid: %v, ptr: %v}", h.attached, h.filename, h.key, h.size, h.shmid, h.ptr)
}

// CreateShm creates a struct VShmHandler
func (h *VShmHandler) CreateShm(pathname string, size uintptr) error {
	if h.attached {
		return &errorString{"This handler is already attached."}
	}
	if size < 1 {
		return &errorString{"The size is invalid."}
	}
	var err error
	err = generateShmJSONFile(pathname, 1, size, 1)
	if err != nil {
		return err
	}
	h.key, err = ftok(pathname)
	if err != nil {
		return &errorString{"ftok Error."}
	}
	tempShmID, _, er := unix.Syscall(unix.SYS_SHMGET, h.key, size, 01000|0777)
	if er != 0 {
		return err
	}
	h.shmid = tempShmID
	h.size = size
	err = generateShmJSONFile(pathname, h.key, size, h.shmid)
	if err != nil {
		return err
	}
	h.filename = pathname
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
	h.key = datashm.Key
	h.size = datashm.Size
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
	addr, _, err := unix.Syscall(unix.SYS_SHMDT, h.ptr, 0, 0)
	if addr != 0 {
		return err
	}
	h.attached = false
	h.ptr = 0
	return nil
}

// NewVShmHandler returns a VShmHandler
func NewVShmHandler() *VShmHandler {
	return &VShmHandler{false, "", 0, 0, 0, 0}
}

func generateShmJSONFile(pathname string, key, size, shmid uintptr) error {
	if key < 0 {
		return &errorString{"Invalid key"}
	}
	if size < 1 {
		return &errorString{"Invalid Size"}
	}
	if shmid < 0 {
		return &errorString{"Invalid SHMID"}
	}
	info := &vShmMarshalStr{key, size, shmid}

	bytes, ok := json.Marshal(info)
	if ok != nil {
		return ok
	}
	err := ioutil.WriteFile(pathname, bytes, 0777)
	if err != nil {
		return err
	}
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
