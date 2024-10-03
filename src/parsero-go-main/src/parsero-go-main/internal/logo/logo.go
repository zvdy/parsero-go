// internal/logo/logo.go
package logo

import (
	"fmt"

	"github.com/zvdy/parsero-go/pkg/colors"
)

func PrintLogo() {
	hello := `
      ____
     |  _ \ __ _ _ __ ___  ___ _ __ ___  
     | |_) / _` + "`" + ` | '__/ __|/ _ \ '__/ _ \ 
     |  __/ (_| | |  \__ \  __/ | | (_) |
     |_|   \__,_|_|  |___/\___|_|  \___/ 
    `
	fmt.Println(colors.YELLOW + hello + colors.ENDC)
}
