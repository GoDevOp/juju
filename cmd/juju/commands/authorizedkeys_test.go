// Copyright 2012, 2013 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package commands

import (
	"fmt"
	"strings"

	jc "github.com/juju/testing/checkers"
	gc "gopkg.in/check.v1"

	keymanagerserver "github.com/juju/juju/apiserver/keymanager"
	keymanagertesting "github.com/juju/juju/apiserver/keymanager/testing"
	"github.com/juju/juju/juju/osenv"
	jujutesting "github.com/juju/juju/juju/testing"
	coretesting "github.com/juju/juju/testing"
	"github.com/juju/juju/testing/factory"
	sshtesting "github.com/juju/juju/utils/ssh/testing"
)

type AuthorizedKeysSuite struct {
	coretesting.FakeJujuHomeSuite
}

var _ = gc.Suite(&AuthorizedKeysSuite{})

var authKeysCommandNames = []string{
	"add",
	"delete",
	"help",
	"import",
	"list",
}

func (s *AuthorizedKeysSuite) TestHelpCommands(c *gc.C) {
	// Check that we have correctly registered all the sub commands
	// by checking the help output.
	out := badrun(c, 0, "authorized-keys", "--help")
	lines := strings.Split(out, "\n")
	var names []string
	subcommandsFound := false
	for _, line := range lines {
		f := strings.Fields(line)
		if len(f) == 1 && f[0] == "commands:" {
			subcommandsFound = true
			continue
		}
		if !subcommandsFound || len(f) == 0 || !strings.HasPrefix(line, "    ") {
			continue
		}
		names = append(names, f[0])
	}
	// The names should be output in alphabetical order, so don't sort.
	c.Assert(names, gc.DeepEquals, authKeysCommandNames)
}

func (s *AuthorizedKeysSuite) assertHelpOutput(c *gc.C, cmd, args string) {
	if args != "" {
		args = " " + args
	}
	expected := fmt.Sprintf("usage: juju authorized-keys %s [options]%s", cmd, args)
	out := badrun(c, 0, "authorized-keys", cmd, "--help")
	lines := strings.Split(out, "\n")
	c.Assert(lines[0], gc.Equals, expected)
}

func (s *AuthorizedKeysSuite) TestHelpList(c *gc.C) {
	s.assertHelpOutput(c, "list", "")
}

func (s *AuthorizedKeysSuite) TestHelpAdd(c *gc.C) {
	s.assertHelpOutput(c, "add", "<ssh key> [...]")
}

func (s *AuthorizedKeysSuite) TestHelpDelete(c *gc.C) {
	s.assertHelpOutput(c, "delete", "<ssh key id> [...]")
}

func (s *AuthorizedKeysSuite) TestHelpImport(c *gc.C) {
	s.assertHelpOutput(c, "import", "<ssh key id> [...]")
}

type keySuiteBase struct {
	jujutesting.JujuConnSuite
	CmdBlockHelper
}

func (s *keySuiteBase) SetUpSuite(c *gc.C) {
	s.JujuConnSuite.SetUpSuite(c)
	s.PatchEnvironment(osenv.JujuEnvEnvKey, "dummyenv")
}

func (s *keySuiteBase) SetUpTest(c *gc.C) {
	s.JujuConnSuite.SetUpTest(c)
	s.CmdBlockHelper = NewCmdBlockHelper(s.APIState)
	c.Assert(s.CmdBlockHelper, gc.NotNil)
	s.AddCleanup(func(*gc.C) { s.CmdBlockHelper.Close() })
}

func (s *keySuiteBase) setAuthorizedKeys(c *gc.C, keys ...string) {
	keyString := strings.Join(keys, "\n")
	err := s.State.UpdateEnvironConfig(map[string]interface{}{"authorized-keys": keyString}, nil, nil)
	c.Assert(err, jc.ErrorIsNil)
	envConfig, err := s.State.EnvironConfig()
	c.Assert(err, jc.ErrorIsNil)
	c.Assert(envConfig.AuthorizedKeys(), gc.Equals, keyString)
}

func (s *keySuiteBase) assertEnvironKeys(c *gc.C, expected ...string) {
	envConfig, err := s.State.EnvironConfig()
	c.Assert(err, jc.ErrorIsNil)
	keys := envConfig.AuthorizedKeys()
	c.Assert(keys, gc.Equals, strings.Join(expected, "\n"))
}

type ListKeysSuite struct {
	keySuiteBase
}

var _ = gc.Suite(&ListKeysSuite{})

func (s *ListKeysSuite) TestListKeys(c *gc.C) {
	key1 := sshtesting.ValidKeyOne.Key + " user@host"
	key2 := sshtesting.ValidKeyTwo.Key + " another@host"
	s.setAuthorizedKeys(c, key1, key2)

	context, err := coretesting.RunCommand(c, newListKeysCommand())
	c.Assert(err, jc.ErrorIsNil)
	output := strings.TrimSpace(coretesting.Stdout(context))
	c.Assert(err, jc.ErrorIsNil)
	c.Assert(output, gc.Matches, "Keys for user admin:\n.*\\(user@host\\)\n.*\\(another@host\\)")
}

func (s *ListKeysSuite) TestListFullKeys(c *gc.C) {
	key1 := sshtesting.ValidKeyOne.Key + " user@host"
	key2 := sshtesting.ValidKeyTwo.Key + " another@host"
	s.setAuthorizedKeys(c, key1, key2)

	context, err := coretesting.RunCommand(c, newListKeysCommand(), "--full")
	c.Assert(err, jc.ErrorIsNil)
	output := strings.TrimSpace(coretesting.Stdout(context))
	c.Assert(err, jc.ErrorIsNil)
	c.Assert(output, gc.Matches, "Keys for user admin:\n.*user@host\n.*another@host")
}

func (s *ListKeysSuite) TestListKeysNonDefaultUser(c *gc.C) {
	key1 := sshtesting.ValidKeyOne.Key + " user@host"
	key2 := sshtesting.ValidKeyTwo.Key + " another@host"
	s.setAuthorizedKeys(c, key1, key2)
	s.Factory.MakeUser(c, &factory.UserParams{Name: "fred"})

	context, err := coretesting.RunCommand(c, newListKeysCommand(), "--user", "fred")
	c.Assert(err, jc.ErrorIsNil)
	output := strings.TrimSpace(coretesting.Stdout(context))
	c.Assert(err, jc.ErrorIsNil)
	c.Assert(output, gc.Matches, "Keys for user fred:\n.*\\(user@host\\)\n.*\\(another@host\\)")
}

func (s *ListKeysSuite) TestTooManyArgs(c *gc.C) {
	_, err := coretesting.RunCommand(c, newListKeysCommand(), "foo")
	c.Assert(err, gc.ErrorMatches, `unrecognized args: \["foo"\]`)
}

type AddKeySuite struct {
	keySuiteBase
}

var _ = gc.Suite(&AddKeySuite{})

func (s *AddKeySuite) TestAddKey(c *gc.C) {
	key1 := sshtesting.ValidKeyOne.Key + " user@host"
	s.setAuthorizedKeys(c, key1)

	key2 := sshtesting.ValidKeyTwo.Key + " another@host"
	context, err := coretesting.RunCommand(c, newAddKeysCommand(), key2, "invalid-key")
	c.Assert(err, jc.ErrorIsNil)
	c.Assert(coretesting.Stderr(context), gc.Matches, `cannot add key "invalid-key".*\n`)
	s.assertEnvironKeys(c, key1, key2)
}

func (s *AddKeySuite) TestBlockAddKey(c *gc.C) {
	key1 := sshtesting.ValidKeyOne.Key + " user@host"
	s.setAuthorizedKeys(c, key1)

	key2 := sshtesting.ValidKeyTwo.Key + " another@host"
	// Block operation
	s.BlockAllChanges(c, "TestBlockAddKey")
	_, err := coretesting.RunCommand(c, newAddKeysCommand(), key2, "invalid-key")
	s.AssertBlocked(c, err, ".*TestBlockAddKey.*")
}

func (s *AddKeySuite) TestAddKeyNonDefaultUser(c *gc.C) {
	key1 := sshtesting.ValidKeyOne.Key + " user@host"
	s.setAuthorizedKeys(c, key1)
	s.Factory.MakeUser(c, &factory.UserParams{Name: "fred"})

	key2 := sshtesting.ValidKeyTwo.Key + " another@host"
	context, err := coretesting.RunCommand(c, newAddKeysCommand(), "--user", "fred", key2)
	c.Assert(err, jc.ErrorIsNil)
	c.Assert(coretesting.Stderr(context), gc.Equals, "")
	s.assertEnvironKeys(c, key1, key2)
}

type DeleteKeySuite struct {
	keySuiteBase
}

var _ = gc.Suite(&DeleteKeySuite{})

func (s *DeleteKeySuite) TestDeleteKeys(c *gc.C) {
	key1 := sshtesting.ValidKeyOne.Key + " user@host"
	key2 := sshtesting.ValidKeyTwo.Key + " another@host"
	s.setAuthorizedKeys(c, key1, key2)

	context, err := coretesting.RunCommand(c, newDeleteKeysCommand(),
		sshtesting.ValidKeyTwo.Fingerprint, "invalid-key")
	c.Assert(err, jc.ErrorIsNil)
	c.Assert(coretesting.Stderr(context), gc.Matches, `cannot delete key id "invalid-key".*\n`)
	s.assertEnvironKeys(c, key1)
}

func (s *DeleteKeySuite) TestBlockDeleteKeys(c *gc.C) {
	key1 := sshtesting.ValidKeyOne.Key + " user@host"
	key2 := sshtesting.ValidKeyTwo.Key + " another@host"
	s.setAuthorizedKeys(c, key1, key2)

	// Block operation
	s.BlockAllChanges(c, "TestBlockDeleteKeys")
	_, err := coretesting.RunCommand(c, newDeleteKeysCommand(),
		sshtesting.ValidKeyTwo.Fingerprint, "invalid-key")
	s.AssertBlocked(c, err, ".*TestBlockDeleteKeys.*")
}

func (s *DeleteKeySuite) TestDeleteKeyNonDefaultUser(c *gc.C) {
	key1 := sshtesting.ValidKeyOne.Key + " user@host"
	key2 := sshtesting.ValidKeyTwo.Key + " another@host"
	s.setAuthorizedKeys(c, key1, key2)
	s.Factory.MakeUser(c, &factory.UserParams{Name: "fred"})

	context, err := coretesting.RunCommand(c, newDeleteKeysCommand(),
		"--user", "fred", sshtesting.ValidKeyTwo.Fingerprint)
	c.Assert(err, jc.ErrorIsNil)
	c.Assert(coretesting.Stderr(context), gc.Equals, "")
	s.assertEnvironKeys(c, key1)
}

type ImportKeySuite struct {
	keySuiteBase
}

var _ = gc.Suite(&ImportKeySuite{})

func (s *ImportKeySuite) SetUpTest(c *gc.C) {
	s.keySuiteBase.SetUpTest(c)
	s.PatchValue(&keymanagerserver.RunSSHImportId, keymanagertesting.FakeImport)
}

func (s *ImportKeySuite) TestImportKeys(c *gc.C) {
	key1 := sshtesting.ValidKeyOne.Key + " user@host"
	s.setAuthorizedKeys(c, key1)

	context, err := coretesting.RunCommand(c, newImportKeysCommand(), "lp:validuser", "invalid-key")
	c.Assert(err, jc.ErrorIsNil)
	c.Assert(coretesting.Stderr(context), gc.Matches, `cannot import key id "invalid-key".*\n`)
	s.assertEnvironKeys(c, key1, sshtesting.ValidKeyThree.Key)
}

func (s *ImportKeySuite) TestBlockImportKeys(c *gc.C) {
	key1 := sshtesting.ValidKeyOne.Key + " user@host"
	s.setAuthorizedKeys(c, key1)

	// Block operation
	s.BlockAllChanges(c, "TestBlockImportKeys")
	_, err := coretesting.RunCommand(c, newImportKeysCommand(), "lp:validuser", "invalid-key")
	s.AssertBlocked(c, err, ".*TestBlockImportKeys.*")
}

func (s *ImportKeySuite) TestImportKeyNonDefaultUser(c *gc.C) {
	key1 := sshtesting.ValidKeyOne.Key + " user@host"
	s.setAuthorizedKeys(c, key1)
	s.Factory.MakeUser(c, &factory.UserParams{Name: "fred"})

	context, err := coretesting.RunCommand(c, newImportKeysCommand(), "--user", "fred", "lp:validuser")
	c.Assert(err, jc.ErrorIsNil)
	c.Assert(coretesting.Stderr(context), gc.Equals, "")
	s.assertEnvironKeys(c, key1, sshtesting.ValidKeyThree.Key)
}
