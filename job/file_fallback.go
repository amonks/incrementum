package job

import "os"

func readFileWithFallback(path, fallbackPath string) ([]byte, string, error) {
	data, err := os.ReadFile(path)
	if err == nil {
		return data, path, nil
	}
	if !os.IsNotExist(err) || fallbackPath == "" {
		return nil, "", err
	}

	data, err = os.ReadFile(fallbackPath)
	if err != nil {
		return nil, "", err
	}
	return data, fallbackPath, nil
}
